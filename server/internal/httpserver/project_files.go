package httpserver

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Server) ProjectFileRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/items/{projectId}/files", s.requireUser(s.listProjectFiles))
	mux.HandleFunc("GET /api/v1/spaces/{spaceId}/files", s.requireUser(s.listSpaceFiles))
}

// fileCandidates bounds one listing before the permission filter.
const fileCandidates = 5000

// subtreeCTE walks an item and its live descendants (at most 30 levels), keeping the parent each one was reached from.
const subtreeCTE = `WITH RECURSIVE t(id,parent,depth) AS (
 SELECT $1::uuid,NULL::uuid,0
 UNION SELECT l.item_id,l.parent_item_id,t.depth+1 FROM t JOIN item_locations l ON l.parent_item_id=t.id AND l.deleted_root_id IS NULL
  JOIN items i ON i.id=l.item_id AND i.deleted_at IS NULL WHERE t.depth<30)`

// spaceCTE is every live item with a place in a Space, with its parent there.
const spaceCTE = `WITH t(id,parent,depth) AS (
 SELECT l.item_id,l.parent_item_id,0 FROM item_locations l JOIN items i ON i.id=l.item_id AND i.deleted_at IS NULL
 WHERE l.space_id=$1::uuid AND l.deleted_root_id IS NULL)`

// fileScope says whose files a listing gathers.
type fileScope struct {
	cte    string    // defines t(id,parent,depth) from $1
	id     uuid.UUID // $1
	root   string    // the item being viewed; empty for a Space, which holds no files itself
	within *string   // judge permissions through this Space only
}

type filesQuery struct {
	q, scope, kind, review, project string
	limit, offset                   int // limit 0 = every match
}

func parseFilesQuery(v url.Values) (filesQuery, bool) {
	out := filesQuery{q: strings.ToLower(strings.TrimSpace(v.Get("q"))), scope: v.Get("scope"), kind: v.Get("kind"), review: v.Get("review"), project: v.Get("project")}
	ok := len(out.q) <= 200 &&
		slices.Contains([]string{"", "direct", "subitems"}, out.scope) &&
		slices.Contains([]string{"", "file", "link"}, out.kind) &&
		slices.Contains([]string{"", "not_requested", "in_review", "approved", "rejected", "cancelled"}, out.review)
	if out.project != "" {
		id, err := uuid.Parse(out.project)
		ok = ok && err == nil
		out.project = id.String()
	}
	number := func(name string, min, max int, into *int) {
		if !v.Has(name) {
			return
		}
		n, err := strconv.Atoi(v.Get(name))
		ok = ok && err == nil && n >= min && n <= max
		*into = n
	}
	number("limit", 1, 200, &out.limit)
	number("offset", 0, fileCandidates, &out.offset)
	return out, ok
}

type fileNode struct{ parent, title, kind string }

// ancestry returns the items from just below root down to id, outermost first.
func ancestry(nodes map[string]fileNode, root, id string) []string {
	var chain []string
	for seen := map[string]bool{}; id != root && id != "" && !seen[id]; id = nodes[id].parent {
		seen[id] = true
		chain = append(chain, id)
	}
	slices.Reverse(chain)
	return chain
}

// listProjectFiles lists live attachments of a project or folder and its descendants, only for items the caller can view. Counts and
// the project list cover every visible file; q, scope, kind, review and project narrow the rows, limit and offset page them. A
// location never names an item the caller cannot view.
// shortcut: newest 5000 candidates, filtered and paged in memory; `truncated` reports the bound. Move to keyset paging in SQL
// when a folder outgrows it.
func (s *Server) listProjectFiles(w http.ResponseWriter, r *http.Request) {
	project, err := uuid.Parse(r.PathValue("projectId"))
	if err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	s.listFiles(w, r, fileScope{cte: subtreeCTE, id: project, root: project.String()}, func(ctx context.Context, tx pgx.Tx) error {
		_, err := approvalCaps(ctx, tx, actorOf(r), project, policy.View)
		return err
	})
}

// listSpaceFiles lists the attachments of every item in a Space. It needs the Space itself: a share on single items is not enough.
func (s *Server) listSpaceFiles(w http.ResponseWriter, r *http.Request) {
	space, err := uuid.Parse(r.PathValue("spaceId"))
	if err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	id := space.String()
	s.listFiles(w, r, fileScope{cte: spaceCTE, id: space, within: &id}, func(ctx context.Context, tx pgx.Tx) error {
		access, found, err := policy.SpaceAccessFor(ctx, tx, actorOf(r), id)
		if err == nil && (!found || !access.Caps.Has(policy.View)) {
			err = items.ErrNotFound
		}
		return err
	})
}

// listFiles answers both listings once gate has let the viewer into the scope.
func (s *Server) listFiles(w http.ResponseWriter, r *http.Request, scope fileScope, gate func(context.Context, pgx.Tx) error) {
	query, ok := parseFilesQuery(r.URL.Query())
	if !ok {
		writeError(w, http.StatusUnprocessableEntity, "invalid file filter or page")
		return
	}
	ctx := r.Context()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	actor := actorOf(r)
	if err := gate(ctx, tx); err != nil {
		writeItemError(w, err)
		return
	}
	rows, err := tx.Query(ctx, scope.cte+`
 , outcomes(attachment_id,attachment_version,created_at,outcome) AS (
  -- WA-D08: one outcome per approval pinned to a file version, using the same rule as the approval list.
  SELECT a.attachment_id,a.attachment_version,a.created_at,(`+approvalOutcome+`)
  FROM approvals a JOIN t ON t.id=a.item_id WHERE a.attachment_id IS NOT NULL)
 SELECT a.item_id::text, jsonb_build_object('id',a.id,'item_id',a.item_id,'item_title',i.title,'filename',a.filename,
 'content_type',a.content_type,'size_bytes',a.size_bytes,'created_at',a.created_at,
 'version',a.current_version,'link_url',a.link_url,
 'review_status',CASE WHEN r.outcome IS NULL THEN
   CASE WHEN EXISTS(SELECT 1 FROM approval_events e WHERE e.kind='withdrawn' AND e.attachment_id=a.id AND e.attachment_version=a.current_version) THEN 'cancelled' ELSE 'not_requested' END
  WHEN r.outcome='pending' THEN 'in_review' ELSE r.outcome END,
 'uploader',jsonb_build_object('id',u.id,'name',concat_ws(' ',u.first_name,u.last_name)))
 FROM attachments a JOIN (SELECT DISTINCT id FROM t) t ON t.id=a.item_id
 JOIN items i ON i.id=a.item_id AND i.deleted_at IS NULL JOIN users u ON u.id=a.uploader_id
 LEFT JOIN LATERAL (SELECT o.outcome FROM outcomes o WHERE o.attachment_id=a.id AND o.attachment_version=a.current_version
   ORDER BY (o.outcome='pending') DESC, o.created_at DESC LIMIT 1) r ON TRUE
 WHERE a.deleted_at IS NULL ORDER BY a.created_at DESC,a.id LIMIT $2`, scope.id, fileCandidates)
	if err != nil {
		writeItemError(w, err)
		return
	}
	type row struct {
		ItemID string
		Body   json.RawMessage
	}
	all, err := pgx.CollectRows(rows, pgx.RowToStructByPos[row])
	if err != nil {
		writeItemError(w, err)
		return
	}
	// The shallowest way to each item gives its location.
	nodeRows, err := tx.Query(ctx, scope.cte+`
 SELECT DISTINCT ON (t.id) t.id::text, coalesce(t.parent::text,''), i.title, i.type::text
 FROM t JOIN items i ON i.id=t.id ORDER BY t.id,t.depth,t.parent NULLS FIRST`, scope.id)
	if err != nil {
		writeItemError(w, err)
		return
	}
	nodes := map[string]fileNode{}
	var nodeID string
	var node fileNode
	if _, err := pgx.ForEachRow(nodeRows, []any{&nodeID, &node.parent, &node.title, &node.kind}, func() error {
		nodes[nodeID] = node
		return nil
	}); err != nil {
		writeItemError(w, err)
		return
	}
	root := scope.root
	var ids []string
	asked := map[string]bool{}
	chains := make([][]string, len(all))
	for i, f := range all {
		chains[i] = ancestry(nodes, root, f.ItemID)
		for _, id := range append([]string{f.ItemID}, chains[i]...) {
			if !asked[id] {
				asked[id] = true
				ids = append(ids, id)
			}
		}
	}
	caps, err := policy.ItemsCaps(ctx, tx, actor, ids, scope.within)
	if err != nil {
		writeItemError(w, err)
		return
	}
	type projectFiles struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Files int    `json:"files"`
	}
	var direct, visible, total int
	byProject := map[string]*projectFiles{}
	out := []map[string]any{}
	for n, f := range all {
		if !caps[f.ItemID].Has(policy.View) {
			continue
		}
		var fields struct {
			Filename     string  `json:"filename"`
			LinkURL      *string `json:"link_url"`
			ReviewStatus string  `json:"review_status"`
			Uploader     struct {
				ID string `json:"id"`
			} `json:"uploader"`
		}
		var body map[string]any
		if err := errors.Join(json.Unmarshal(f.Body, &fields), json.Unmarshal(f.Body, &body)); err != nil {
			writeItemError(w, err)
			return
		}
		path, nearest := []string{}, ""
		for _, id := range chains[n] {
			if !caps[id].Has(policy.View) {
				continue
			}
			path = append(path, nodes[id].title)
			if nodes[id].kind == "project" {
				nearest = id
			}
		}
		visible++
		if f.ItemID == root {
			direct++
		}
		if nearest != "" {
			if byProject[nearest] == nil {
				byProject[nearest] = &projectFiles{ID: nearest, Title: nodes[nearest].title}
			}
			byProject[nearest].Files++
		}
		if (query.q != "" && !strings.Contains(strings.ToLower(fields.Filename), query.q)) ||
			(query.scope == "direct" && f.ItemID != root) || (query.scope == "subitems" && f.ItemID == root) ||
			(query.kind == "file" && fields.LinkURL != nil) || (query.kind == "link" && fields.LinkURL == nil) ||
			(query.review != "" && fields.ReviewStatus != query.review) || (query.project != "" && nearest != query.project) {
			continue
		}
		total++
		if total > query.offset && (query.limit == 0 || len(out) < query.limit) {
			body["path"] = path
			// The same rules as the attachment endpoints, which check them again.
			attach := caps[f.ItemID].Has(policy.Attach)
			body["can_delete"] = caps[f.ItemID].Has(policy.Delete) || (attach && fields.Uploader.ID == actor.ID)
			body["can_upload_version"] = attach && fields.LinkURL == nil
			out = append(out, body)
		}
	}
	projects := []projectFiles{}
	for _, p := range byProject {
		projects = append(projects, *p)
	}
	slices.SortFunc(projects, func(a, b projectFiles) int {
		return cmp.Or(strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title)), strings.Compare(a.ID, b.ID))
	})
	writeJSON(w, http.StatusOK, map[string]any{"items": out, "total": total, "projects": projects, "truncated": len(all) == fileCandidates,
		"counts": map[string]int{"all": visible, "direct": direct, "subitems": visible - direct, "projects": len(projects)}})
}
