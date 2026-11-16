import json,os,qa_http
CFG=json.load(open("/Volumes/MyHDD/homeserver/apps/pms/docs/qa/ PMS_ITA_PHASE1_SEED/seed_config.json"))
PW=open("/private/tmp/claude-501/qa/seed_password").read().strip(); MAP="/private/tmp/claude-501/qa/keymap.json"; km=json.load(open(MAP)); log=[]
a=qa_http.C(); assert a.call("POST","/api/v1/auth/login",{"email":"admin@pms-ita.test","password":PW})[0]==200
def save(): json.dump(km,open(MAP,"w"),indent=1)
def ok(r,w):
    if r[0] not in (200,201,204): log.append(f"FAIL {w}: {r[0]} {str(r[1])[:200]}"); return None
    return r[1]
ROLE={"Full":"full","Editor":"editor","Limited":"limited","Read Only":"read"}
# statuses of default workflow
wf=a.call("GET","/api/v1/workflows")[1]; wfl=wf.get("items",wf) if isinstance(wf,dict) else wf
dflt=[w for w in wfl if w.get("is_default")][0]; st={s["name"]:s["id"] for s in dflt["statuses"]}
SM={"Backlog":"New","In Progress":"In Progress","Completed":"Completed","In Review":"In Progress","Blocked":"On Hold"}
items=km['items']
ROLE={'Full':'full','Editor':'editor','Limited':'limited','Read Only':'read'}
def pr(p):
    return ('group',km['groups'][p]) if p in km['groups'] else ('user',km['users'][p])

# shares on items/spaces
for sh in CFG["sharing"]:
    t=items.get(sh["target"]); 
    if not t: continue
    pt,pid=pr(sh["principal"])
    r=a.call("GET",f"/api/v1/items/{t}/shares"); etag=r[2].get("ETag")
    cur=(r[1] or {}).get("shares",(r[1] or {}).get("items",[])) if r[0]==200 else []
    cur=[{"principal_type":x["principal_type"],"principal_id":x["principal_id"],"role":x["role"]} for x in cur]
    if not any(c["principal_id"]==pid for c in cur):
        cur.append({"principal_type":pt,"principal_id":pid,"role":ROLE[sh["role"]]})
        ok(a.call("PUT",f"/api/v1/items/{t}/shares",{"shares":cur},{"If-Match":etag}),f"share {sh['target']}/{sh['principal']}")
print("items",len(items),"spaces",sp); print("\n".join(log[:25]) or "ok"); open("stageB_log.txt","w").write("\n".join(log))
