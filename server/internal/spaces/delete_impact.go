package spaces

import (
	"context"

	"github.com/anissaefuddin/pms/server/internal/policy"
)

// DeleteImpact counts what deleting a Space would affect (D). Nothing is changed by reading it. Items that live in
// another Space too are not removed by the delete; they are counted separately.
type DeleteImpact struct {
	ProjectsAndFolders int `json:"projects_and_folders"`
	ItemsOnlyHere      int `json:"items_only_here"`
	ItemsElsewhere     int `json:"items_elsewhere"`
	Members            int `json:"members"`
	Groups             int `json:"groups"`
	AutomationRules    int `json:"automation_rules"`
	RequestForms       int `json:"request_forms"`
	Blueprints         int `json:"blueprints"`
	Tools              int `json:"tools"`
	Admins             int `json:"space_admins"`
}

// GetDeleteImpact needs the Delete capability; a person who cannot delete the Space gets 404 for its impact too.
func (s *Service) GetDeleteImpact(ctx context.Context, actor policy.Actor, id string) (DeleteImpact, error) {
	if !IsID(id) {
		return DeleteImpact{}, ErrNotFound
	}
	decision, access, err := policy.CanOnSpace(ctx, s.pool, actor, policy.Delete, id)
	if err != nil {
		return DeleteImpact{}, err
	}
	if decision == policy.NotFound {
		return DeleteImpact{}, ErrNotFound
	}
	if access.Kind == "personal" || decision != policy.Allowed {
		return DeleteImpact{}, forbidden("you cannot delete this Space")
	}
	var out DeleteImpact
	err = s.pool.QueryRow(ctx, `SELECT
  (SELECT count(DISTINCT l.item_id) FROM item_locations l JOIN items i ON i.id=l.item_id AND i.deleted_at IS NULL
     WHERE l.space_id=$1::uuid AND l.deleted_root_id IS NULL AND i.type IN ('project','folder')),
  (SELECT count(*) FROM item_locations l JOIN items i ON i.id=l.item_id AND i.deleted_at IS NULL
     WHERE l.space_id=$1::uuid AND l.deleted_root_id IS NULL
       AND NOT EXISTS (SELECT 1 FROM item_locations o JOIN spaces s ON s.id=o.space_id
         WHERE o.item_id=l.item_id AND o.deleted_root_id IS NULL AND o.space_id<>$1::uuid AND s.deleted_at IS NULL)),
  (SELECT count(DISTINCT l.item_id) FROM item_locations l JOIN items i ON i.id=l.item_id AND i.deleted_at IS NULL
     WHERE l.space_id=$1::uuid AND l.deleted_root_id IS NULL
       AND EXISTS (SELECT 1 FROM item_locations o JOIN spaces s ON s.id=o.space_id
         WHERE o.item_id=l.item_id AND o.deleted_root_id IS NULL AND o.space_id<>$1::uuid AND s.deleted_at IS NULL)),
  (SELECT count(*) FROM shares WHERE target_type='space' AND target_id=$1::uuid AND principal_type='user'),
  (SELECT count(*) FROM shares WHERE target_type='space' AND target_id=$1::uuid AND principal_type='group'),
  (SELECT count(*) FROM automation_rules WHERE space_id=$1::uuid),
  (SELECT count(*) FROM request_forms WHERE space_id=$1::uuid),
  (SELECT count(*) FROM blueprints WHERE space_id=$1::uuid),
  (SELECT count(*) FROM views WHERE scope_space_id=$1::uuid AND scope_item_id IS NULL),
  (SELECT count(*) FROM space_admins WHERE space_id=$1::uuid)`, id).Scan(
		&out.ProjectsAndFolders, &out.ItemsOnlyHere, &out.ItemsElsewhere, &out.Members, &out.Groups,
		&out.AutomationRules, &out.RequestForms, &out.Blueprints, &out.Tools, &out.Admins)
	return out, err
}
