import json,os,urllib.request,http.cookiejar
BASE=os.environ.get("QA_BASE","https://dev-lens.losakaenergi.app")
class C:
    def __init__(s):
        s.cj=http.cookiejar.CookieJar(); s.o=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(s.cj))
    def call(s,m,p,body=None,h=None):
        hd={"Content-Type":"application/json","User-Agent":"Mozilla/5.0 pms-qa","Origin":BASE}; hd.update(h or {})
        r=urllib.request.Request(BASE+p,json.dumps(body).encode() if body is not None else None,hd,method=m)
        try:
            x=s.o.open(r); t=x.read(); return x.status,(json.loads(t) if t else None),x.headers
        except urllib.error.HTTPError as e:
            t=e.read()
            try: return e.code,json.loads(t),e.headers
            except Exception: return e.code,t[:200],e.headers
