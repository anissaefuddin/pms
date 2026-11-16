package httpserver_test

import "testing"

// WA-D08: review status belongs to the file version. A pending review shows "in_review"; a withdrawn review shows
// "cancelled" for its version; a new version with no review shows "not_requested" and never inherits an old result.
func TestProjectFilesReviewStatusIsPerVersion(t *testing.T) {
	f := newFixture(t)
	admin, approver := f.person("admin"), f.person("regular")
	space := f.space("Review status")
	f.share(space, "user", approver.id, "read")
	project := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','Project',$1) RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", project, space)
	task := customFieldItem(f, admin, space, "Spec")
	f.exec("UPDATE item_locations SET parent_item_id=$1 WHERE item_id=$2 AND space_id=$3", project, task, space)
	file := f.queryID(`INSERT INTO attachments(id,item_id,uploader_id,filename,content_type,size_bytes,storage_key,sha256)
VALUES (gen_random_uuid(),$1,$2,'spec.txt','text/plain',1,'review-k1',decode(repeat('00',32),'hex')) RETURNING id::text`, task, admin.id)
	f.exec(`INSERT INTO attachment_versions(attachment_id,version,uploader_id,filename,content_type,size_bytes,storage_key,sha256)
VALUES ($1,1,$2,'spec.txt','text/plain',1,'review-v1',decode(repeat('00',32),'hex'))`, file, admin.id)
	approval := f.queryID(`INSERT INTO approvals(id,item_id,created_by,policy,attachment_id,attachment_version)
VALUES (gen_random_uuid(),$1,$2,'any',$3,1) RETURNING id::text`, task, admin.id, file)
	f.exec("INSERT INTO approval_approvers(approval_id,user_id) VALUES ($1,$2)", approval, approver.id)

	statusOf := func() string {
		for _, item := range f.call(admin, "GET", "/items/"+project+"/files", nil).items() {
			if item["id"] == file {
				return item["review_status"].(string)
			}
		}
		t.Fatalf("file missing from project list")
		return ""
	}
	if got := statusOf(); got != "in_review" {
		t.Fatalf("pending review should show in_review, got %q", got)
	}
	f.want(f.call(admin, "DELETE", "/approvals/"+approval, nil, "If-Match", `"1"`), 204, "withdraw the review")
	if got := statusOf(); got != "cancelled" {
		t.Fatalf("withdrawn review should show cancelled for its version, got %q", got)
	}
	f.exec("INSERT INTO attachment_versions(attachment_id,version,uploader_id,filename,content_type,size_bytes,storage_key,sha256) VALUES ($1,2,$2,'spec-2.txt','text/plain',1,'review-v2',decode(repeat('00',32),'hex'))", file, admin.id)
	f.exec("UPDATE attachments SET current_version=2 WHERE id=$1", file)
	if got := statusOf(); got != "not_requested" {
		t.Fatalf("a new version must not inherit the withdrawn review, got %q", got)
	}
}
