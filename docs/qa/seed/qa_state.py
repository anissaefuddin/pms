"""Buat Playwright storageState per user QA lewat login API (kata sandi dibaca dari file, tidak dicetak).
Pakai: SSL_CERT_FILE=/etc/ssl/cert.pem python3 qa_state.py pm admin psi  -> $QA_STATE_DIR/state_<user>.json"""
import sys,os,json,qa_http
OUT=os.environ.get("QA_STATE_DIR","/private/tmp/claude-501/qa")
PW=open("/private/tmp/claude-501/qa/seed_password").read().strip()
for u in sys.argv[1:]:
    c=qa_http.C(); s=c.call("POST","/api/v1/auth/login",{"email":u+"@pms-ita.test","password":PW})[0]; assert s==200,(u,s)
    ck=[{"name":x.name,"value":x.value,"domain":x.domain,"path":"/","httpOnly":True,"secure":True,"sameSite":"Lax","expires":int(x.expires or 2000000000)} for x in c.cj]
    json.dump({"cookies":ck,"origins":[]},open(f"{OUT}/state_{u}.json","w")); print(u,"ok")
