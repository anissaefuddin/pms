// Helper Playwright tanpa rahasia. node dari web/: import {open,shot,txt} from '.../docs/qa/seed/ui_helper.mjs'
import {chromium} from '/Volumes/MyHDD/homeserver/apps/pms/web/node_modules/playwright/index.mjs';
import fs from 'fs';
export const BASE='https://dev-lens.losakaenergi.app', EVD='/Volumes/MyHDD/homeserver/apps/pms/docs/qa/evidence/';
export const STATE=u=>`${process.env.QA_STATE_DIR||'/private/tmp/claude-501/qa'}/state_${u}.json`;
export async function open(user,w=1440,h=900){const b=await chromium.launch();const ctx=await b.newContext({viewport:{width:w,height:h},storageState:STATE(user)});const p=await ctx.newPage();p.setDefaultTimeout(4000);return {b,p};}
export async function shot(p,id,name){fs.mkdirSync(EVD+id,{recursive:true});await p.screenshot({path:`${EVD+id}/${name}.png`});return `evidence/${id}/${name}.png`;}
export const txt=async(p,n=600)=>(await p.evaluate(()=>document.body.innerText)).replace(/\s+/g,' ').slice(0,n);
export const axeRun=async p=>{await p.addScriptTag({content:fs.readFileSync('/Volumes/MyHDD/homeserver/apps/pms/web/node_modules/axe-core/axe.min.js','utf8')});return p.evaluate(async()=>(await axe.run()).violations.map(v=>[v.id,v.impact,v.nodes.length]));};
