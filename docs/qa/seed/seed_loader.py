#!/usr/bin/env python3
"""Idempotent QA seed loader (stage A: users, groups, spaces, shares, ITA tree). Uses public API; only invite-token activation touches DB."""
import json,os,subprocess,secrets,sys
import qa_http
CFG=json.load(open("/Volumes/MyHDD/homeserver/apps/pms/docs/qa/ PMS_ITA_PHASE1_SEED/seed_config.json"))
NS=CFG["namespace"]; PW=open("/private/tmp/claude-501/qa/seed_password").read().strip()
MAP="/private/tmp/claude-501/qa/keymap.json"; km=json.load(open(MAP)) if os.path.exists(MAP) else {}
log=[]
def save(): json.dump(km,open(MAP,"w"),indent=1)
def psql(sql):
    return subprocess.run(["ssh","lsuser@100.116.210.127","docker exec -i pms-postgres-1 psql -U pms -d pms -v ON_ERROR_STOP=1 -tA"],input=sql,capture_output=True,text=True)
adm=qa_http.C(); s,b,_=adm.call("POST","/api/v1/auth/login",{"email":"admin@pms-ita.test","password":PW}); assert s==200,(s,b)
def ok(r,what):
    if r[0] not in (200,201,204): log.append(f"FAIL {what}: {r[0]} {str(r[1])[:160]}"); return None
    return r[1]
TYPE={"Admin":"admin","Regular":"regular","External":"external","Collaborator":"collaborator","Viewer":"viewer"}
# users
sessions={}
for u in CFG["users"]:
    k=u["id"]
    if k in km.get("users",{}) and k!="u-admin": continue
    if k=="u-admin": km.setdefault("users",{})[k]=adm.call("GET","/api/v1/me")[1]["id"]; continue
    r=adm.call("POST","/api/v1/users/invitations",{"email":u["email"],"user_type":TYPE[u["user_type"]]})
    if r[0]==409:
        found=[x for x in adm.call("GET","/api/v1/users?limit=200")[1].get("items",[]) if x["email"]==u["email"]]
        uid=found[0]["id"] if found else None
    else:
        uid=(ok(r,"invite "+k) or {}).get("id")
    if not uid: continue
    tok=secrets.token_urlsafe(32)
    p=psql(f"insert into user_tokens(token_hash,user_id,purpose,expires_at) values (sha256(convert_to('{tok}','utf8')),'{uid}','invite',now()+interval '1 hour');")
    import time; time.sleep(25)
    c=qa_http.C(); first,_,last=u["display_name"].partition(" ")
    r=c.call("POST","/api/v1/auth/invitations/accept",{"token":tok,"first_name":first,"last_name":last or "QA","password":PW,"accept_terms":True})
    if ok(r,"accept "+k) is not None: km["users"][k]=uid
    save()
print("users",len(km["users"]))
for u in CFG["users"]:
    if not u["active"] and u["id"] in km["users"]:
        adm.call("POST",f"/api/v1/users/{km['users'][u['id']]}/deactivate",{})
# groups
for g,members in CFG["groups"].items():
    if g in km.setdefault("groups",{}): continue
    r=ok(adm.call("POST","/api/v1/groups",{"name":f"QA-{g}","member_ids":[km["users"][m] for m in members if m in km["users"]]}),"group "+g)
    if r: km["groups"][g]=r["id"]
save()
print("groups",km["groups"])
open("/private/tmp/claude-501/qa/stageA_log.txt","w").write("\n".join(log))
print("\n".join(log) or "stage A part1 ok")
