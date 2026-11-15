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
sp=km.setdefault("spaces",{})
existing={x["name"]:x["id"] for x in a.call("GET","/api/v1/spaces")[1].get("items",[])}
for s in CFG["spaces"]:
    if s["id"] in sp: continue
    if s["name"] in existing: sp[s["id"]]=existing[s["name"]]; continue
    r=ok(a.call("POST","/api/v1/spaces",{"name":s["name"],"description":"QA-ITA-20261115"}),"space "+s["id"])
    if r: sp[s["id"]]=r["id"]
save()
def pr(p):
    return ("group",km["groups"][p]) if p in km["groups"] else ("user",km["users"][p])
# sandbox project containers + items
items=km.setdefault("items",{})
def mk(key,title,typ,parent,space,extra=None):
    if key in items: return items[key]
    body={"title":title,"type":typ,"locations":[{"space_id":sp[space],"parent_item_id":parent}]}; body.update(extra or {})
    r=ok(a.call("POST","/api/v1/items",body),"item "+key)
    if r: items[key]=r["id"]; save()
    return items.get(key)
mk("ITA",CFG["project"]["title"],"project",None,"engineering",{"start_date":CFG["project"]["start"],"due_date":CFG["project"]["due"]})
for t in CFG["timeline_items"]:
    if t["parent_id"] is None: continue
    par=items.get(t["parent_id"]); 
    if t["parent_id"]=="ITA": par=items["ITA"]
    if par is None: log.append("noparent "+t["id"]); continue
    sc=t.get("schedule") or {}; ex={}
    if t["base_type"]=="Task":
        ex={"start_date":sc.get("start"),"due_date":sc.get("due")}
        if t.get("milestone"): ex["is_milestone"]=True
        if t.get("status"): ex["status_id"]=st[SM[t["status"]]]
        ex["assignee_ids"]=[km["users"][u] for u in t.get("assignee_ids",[]) if u in km["users"]]
    mk(t["id"],t["title"],"folder" if t["base_type"]=="Folder" else "task",par,"engineering",ex)
for sb in CFG["sandbox_projects"]:
    mk(sb["id"],sb["name"],"project",None,sb["space"])
for it in CFG["sandbox_items"]:
    mk(it["id"],it["title"],"task",items.get(it["project_id"]),"engineering" if it["project_id"]!="ITA-PRIVATE" else "restricted",{"start_date":it.get("start"),"due_date":it.get("due")})
# shares on items/spaces
for sh in CFG["sharing"]:
    t=items.get(sh["target"]); 
    if not t: continue
    pt,pid=pr(sh["principal"])
    r=a.call("GET",f"/api/v1/items/{t}/shares")
    cur=(r[1] or {}).get("shares",(r[1] or {}).get("items",[])) if r[0]==200 else []
    cur=[{"principal_type":x["principal_type"],"principal_id":x["principal_id"],"role":x["role"]} for x in cur]
    if not any(c["principal_id"]==pid for c in cur):
        cur.append({"principal_type":pt,"principal_id":pid,"role":ROLE[sh["role"]]})
        ok(a.call("PUT",f"/api/v1/items/{t}/shares",{"shares":cur}),f"share {sh['target']}/{sh['principal']}")
print("items",len(items),"spaces",sp); print("\n".join(log[:25]) or "ok"); open("stageB_log.txt","w").write("\n".join(log))
