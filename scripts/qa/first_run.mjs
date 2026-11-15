// ITA-REQ-007: wizard first-run pada instance lokal hasil `seed.sh empty`. Kata sandi dibuat acak per proses, tidak disimpan.
// Pakai dari web/: QA_INSTANCE=a node ../scripts/qa/first_run.mjs <outDir>
import {execSync} from 'child_process';
import fs from 'fs';
import {pathToFileURL} from 'url';
const {chromium}=await import(pathToFileURL(process.cwd()+'/node_modules/playwright/index.mjs').href); // jalankan dari web/
const out=process.argv[2]||'/tmp/first_run'; fs.mkdirSync(out,{recursive:true});
const port=process.env.QA_APP_PORT||(process.env.QA_INSTANCE==='b'?18092:18091), BASE=`http://localhost:${port}`;
const log=execSync('sh '+new URL('./seed.sh',import.meta.url).pathname+' bootstrap 2>&1',{encoding:'utf8',env:process.env});
const token=log.match(/invitations\/([A-Za-z0-9_-]+)/)?.[1]; if(!token) throw new Error('token undangan tidak ada: '+log.slice(-300));
const pw='Qa-'+Math.random().toString(36).slice(2)+'-Zz9!'; const res={};
const J=(p,m='GET',b,h={})=>fetch(BASE+'/api/v1'+p,{method:m,headers:{'Content-Type':'application/json',Origin:BASE,...h},body:b&&JSON.stringify(b)});
let r=await J('/auth/invitations/accept','POST',{token,first_name:'QA',last_name:'Admin',password:pw,accept_terms:true}); res.accept=r.status;
const cookie=r.headers.get('set-cookie')?.split(';')[0]; res.reuse_token=(await J('/auth/invitations/accept','POST',{token,first_name:'QA',last_name:'Admin',password:pw,accept_terms:true})).status;
const me=await (await J('/me','GET',undefined,{Cookie:cookie})).json(); res.pending_before=me.onboarding_pending;
const b=await chromium.launch(); const ctx=await b.newContext({viewport:{width:1440,height:900}}); await ctx.addCookies([{name:'pms_session',value:cookie.split('=')[1],url:BASE}]);
const p=await ctx.newPage(); p.setDefaultTimeout(6000); await p.goto(BASE+'/'); await p.waitForTimeout(3000);
res.wizard_visible=await p.getByText('Set up your workspace').count(); await p.screenshot({path:out+'/wizard_step1.png'});
await p.getByRole('textbox').first().fill('QA Org'); await p.getByRole('button',{name:'Next'}).click();
const boxes=p.getByRole('textbox'); await boxes.nth(0).fill('First project'); for(let i=1;i<=3;i++) await boxes.nth(i).fill('Task '+i);
await p.getByRole('button',{name:'Next'}).click(); await p.getByLabel(/Board/).check(); await p.screenshot({path:out+'/wizard_step3.png'});
await p.getByRole('button',{name:'Create project'}).click(); await p.waitForTimeout(2500);
// Resume: muat ulang sebelum Finish. Wizard harus muncul lagi (belum selesai) tanpa menggandakan proyek yang sudah dibuat.
await p.reload(); await p.waitForTimeout(3000); res.wizard_after_reload=await p.getByText('Set up your workspace').count();
const items=async()=>{const s=await (await J('/spaces','GET',undefined,{Cookie:cookie})).json(); let n=0; for(const sp of s.items.filter(x=>x.kind==='shared')){const t=await (await J('/spaces/'+sp.id+'/items?limit=50','GET',undefined,{Cookie:cookie})).json(); n+=(t.items||[]).filter(i=>i.title==='First project').length;} return {spaces:s.items.map(x=>x.name),projects:n};};
res.after_create=await items();
res.skip_or_finish=null; const fin=p.getByRole('button',{name:/Skip setup|Finish/}).first(); if(await fin.count()){await fin.click(); await p.waitForTimeout(2000); res.skip_or_finish='clicked';}
res.pending_after=(await (await J('/me','GET',undefined,{Cookie:cookie})).json()).onboarding_pending; res.after_finish=await items();
await p.screenshot({path:out+'/after_finish.png'}); await b.close();
fs.writeFileSync(out+'/first_run.json',JSON.stringify(res,null,1)); console.log(JSON.stringify(res));
