"""Self-contained HTML, with all trace-derived text inserted as text, not markup."""

import json


PAGE = r"""<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta http-equiv="Content-Security-Policy"
 content="default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data:; connect-src 'none'; base-uri 'none'; form-action 'none'">
<title>AKS boot observatory</title>
<style>
:root{color-scheme:dark;--bg:#0b1220;--card:#152036;--line:#293a55;--text:#e7edf7;--muted:#9bb0cb;--cyan:#59d9dc;--purple:#b99cff}
*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--text);font:14px system-ui,sans-serif}
main{padding:32px}h1{font-size:30px;margin:6px 0}h2{font-size:18px}
.eyebrow{color:var(--cyan);font-size:12px;letter-spacing:2px}.muted{color:var(--muted)}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:16px;margin:26px 0}
.card,section{background:var(--card);border:1px solid var(--line);border-radius:12px;padding:20px;margin-bottom:18px}
.card strong{display:block;font-size:28px;margin-top:8px}.toolbar{display:flex;gap:14px;align-items:center;flex-wrap:wrap}
input,select,button{background:var(--bg);color:var(--text);border:1px solid #49617e;border-radius:6px;padding:8px}
input[type=range]{width:180px}button{cursor:pointer}canvas{width:100%;display:block;touch-action:pan-y}
.timeline-frame{position:relative;margin-top:16px}
.timeline-scroll{height:min(70vh,680px);min-height:220px;overflow:auto;overflow-x:hidden;position:relative;border-top:1px solid var(--line)}
.timeline-spacer{position:relative;min-width:0}.timeline-sticky{position:sticky;top:0;z-index:1}
.timeline-labels{position:absolute;inset:0 auto auto 0;z-index:1;pointer-events:none}
.timeline-row-label{position:absolute;left:0;white-space:nowrap;overflow:hidden;padding-right:6px;background:var(--card);
 color:var(--text);font:12px/20px system-ui,sans-serif;pointer-events:auto;user-select:text;cursor:text}
.timeline-axis{display:block;height:28px}
.timeline-selection{position:absolute;top:0;bottom:0;z-index:2;display:none;pointer-events:none;
 background:#59d9dc33;border-left:1px solid var(--cyan);border-right:1px solid var(--cyan)}
.timeline-resize{height:18px;display:flex;align-items:center;justify-content:center;cursor:ns-resize;touch-action:none;user-select:none}
.timeline-resize::after{content:"";width:48px;height:4px;border-radius:4px;background:#49617e}
.timeline-resize:hover::after,.timeline-resize:focus-visible::after{background:var(--cyan)}
.timeline-resize:focus-visible{outline:2px solid var(--cyan);outline-offset:-2px}
.service-filter{margin-top:12px}.service-filter summary{cursor:pointer}.service-filter-actions{display:flex;gap:8px;margin:8px 0}
.service-filter-list{display:grid;grid-template-columns:repeat(auto-fit,minmax(220px,1fr));gap:4px;max-height:300px;overflow:auto}
.service-filter-option{display:flex;align-items:center;gap:8px;min-width:0}.service-filter-option span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.scroll{overflow:auto;max-height:520px}table{border-collapse:collapse;width:100%;white-space:nowrap}
th,td{text-align:left;padding:10px;border-bottom:1px solid var(--line)}
th{position:sticky;top:0;background:var(--card);color:var(--muted);cursor:pointer}
tbody tr:hover{background:#20314c}li{margin:9px 0}.warning{border-left:3px solid #efb668;padding-left:16px}
pre{white-space:pre-wrap;overflow-wrap:anywhere;color:var(--muted)}.legend{font-size:12px;margin-top:12px;color:var(--muted)}
.milestone-legend{display:flex;flex-wrap:wrap;gap:8px}.milestone-legend button{display:flex;align-items:center;gap:8px;font-size:12px;text-align:left}
.milestone-swatch{width:24px;flex-shrink:0;border-top:2px dashed}
.timeline-tooltip{position:fixed;z-index:20;max-width:min(440px,calc(100vw - 16px));max-height:calc(100vh - 16px);overflow:auto;padding:12px 16px;background:var(--bg);color:var(--text);border:1px solid #7598bb;border-radius:8px;box-shadow:0 6px 24px #0008;white-space:pre-line;overflow-wrap:anywhere;pointer-events:none}
#compatibility{overflow-wrap:anywhere}
.badge{padding:4px 8px;border-radius:5px;background:#243854;display:inline-block;margin:6px 4px 0 0}
@media(max-width:600px){main{padding:14px}.card,section{padding:12px}}
</style></head><body><main>
<header><div class="eyebrow">NODE LIFECYCLE / BOOT OBSERVATORY</div>
<h1>From Linux boot to NodeReady</h1>
<p class="muted">Measured activity, optimization candidates, and explicit coverage limits. No external assets or telemetry.</p>
<div id="identity"></div></header>
<div class="cards" id="cards"></div>
<section><h2>Boot timeline</h2><div class="toolbar">
<label>Capture <select id="capture"></select></label>
<label>Thread filter <input id="filter" placeholder="thread name or ID..."></label>
<label>From <input id="from" type="number" min="0" step="0.1" value="0" style="width:95px"> s</label>
<label>To <input id="to" type="number" min="0" step="0.1" style="width:95px"> s</label>
<button id="reset">Reset view</button></div>
<details class="service-filter"><summary id="service-filter-summary">Systemd service visibility</summary>
<label>Find service <input id="service-filter-search" placeholder="filter service names..."></label>
<div class="service-filter-actions"><button id="show-services">Show all</button><button id="hide-services">Hide all</button></div>
<div id="service-filter-list" class="service-filter-list"></div></details>
<div id="milestone-legend" class="legend milestone-legend" aria-label="Dashed milestone lines"></div>
<div class="timeline-frame"><canvas id="timeline-axis" class="timeline-axis" aria-hidden="true"></canvas>
<div id="timeline-scroll" class="timeline-scroll"><div id="timeline-spacer" class="timeline-spacer">
<div id="timeline-sticky" class="timeline-sticky"><div id="timeline-service-labels" class="timeline-labels" aria-label="Selectable timeline row labels"></div>
<canvas id="timeline" aria-label="Scrollable boot timeline; equivalent timings are available in the tables below"></canvas>
<div id="timeline-selection" class="timeline-selection" aria-hidden="true"></div>
</div></div></div><div id="timeline-resize" class="timeline-resize" role="separator" aria-orientation="horizontal"
 aria-label="Resize timeline height" aria-controls="timeline-scroll" aria-valuemin="220" tabindex="0"></div></div>
<div class="legend">Turquoise bars: intervals when a systemd unit was active, plus instrumented init-aks-cloud task intervals. Gray ticks: process start; cyan ticks: unit activation; purple ticks: process exit; pink ticks: unit inactive. Purple bars: observed CPU occupancy per thread (100 ms bins).
Orange: block I/O events. Green/blue: network TX/RX events (100 ms bins), not packet counts or latency.
Dashed lines: imported milestones and Ready. All matching scheduler threads with activity in the selected time range are shown below the service and event rows, sorted by CPU; use the vertical scrollbar to inspect them.
Empty space can mean no unit activation was recorded. Unit activity is not application readiness.</div>
<p class="muted">Service/task and scheduler-thread rows with no recorded activity in the selected time range are hidden automatically.</p>
<p class="muted">Drag horizontally anywhere in the timeline graph, including empty space, to select a time range; use Reset view to restore the full capture. Drag the handle below the timeline to change its height, or focus it and use the arrow keys. Select any label in the timeline's left column. Hover over an activity or dashed line for details. Click to keep the popup open; click elsewhere or press Escape to close it.
Milestone legend buttons also show details on hover, focus or click.</p>
<p id="detail" class="muted" aria-live="polite"></p></section>
<section><h2>Optimization candidates: observed thread activity</h2>
<p class="muted">Sort by CPU or runnable wait to investigate costly work and contention. These are measurements, not automatic causal conclusions.
Thread lifetimes, boot phases and multi-CPU execution must not be added together.</p>
<p class="muted">Observed span is last minus first thread observation, not its complete lifetime. Orange shows block activity, not request latency.</p>
<p class="muted">Timeline: all matching scheduler threads, sorted by CPU; scroll vertically to inspect them. Table: up to 2,000 matching threads. Full accounting is retained in report.json.</p>
<div class="scroll"><table><thead><tr id="thread-head"></tr></thead><tbody id="threads"></tbody></table></div></section>
<section><h2>Services and semantic milestones</h2><p class="muted">Rows are ordered by start time. A turquoise interval appears only while the unit is recorded active; oneshot starts and exits are markers, not activity bars.</p><div class="scroll"><table>
<thead><tr><th>Name</th><th>Kind / state</th><th>Process start (s)</th><th>Active (s)</th><th>Inactive / exit (s)</th><th>Evidence</th></tr></thead><tbody id="services"></tbody></table></div></section>
<section id="comparison" hidden><h2>Capture comparison</h2>
<p class="muted">Delta = candidate minus baseline. Negative means shorter, not necessarily a statistically established improvement.
Different tracing backends can have different overhead and coverage.</p>
<div id="compatibility"></div>
<div id="pair-comparison"><div class="scroll"><table><thead><tr><th>Measurement</th><th>Baseline (s)</th><th>Candidate (s)</th><th>Delta (s)</th></tr></thead><tbody id="deltas"></tbody></table></div></div>
<div id="repeat-summary" hidden><p class="muted">Each capture remains individually selectable above. Repeated trials share a configuration, but parallel runs and shared infrastructure can affect timings.</p>
<div class="scroll"><table><thead><tr><th>Capture</th><th>Backend</th><th>Boot → Ready (s)</th></tr></thead><tbody id="trial-readiness"></tbody></table></div>
<div class="scroll"><table><thead><tr><th>Backend</th><th>Trials</th><th>Minimum (s)</th><th>Median (s)</th><th>Maximum (s)</th></tr></thead><tbody id="trial-summary"></tbody></table></div>
<h3>Signals recurring across ftrace captures</h3><p class="muted">Counts describe recorded events; systemd service values are active durations, while instrumented task values are elapsed intervals. A recurring signal is present in at least two captures, not proof of a shared cause.</p>
<div class="scroll"><table><thead><tr><th>Signal</th><th>Captures</th><th>Minimum</th><th>Median</th><th>Maximum</th></tr></thead><tbody id="repeat-patterns"></tbody></table></div></div></section>
<section><h2>Coverage and interpretation</h2><ul class="warning" id="warnings"></ul>
<details><summary>Certificate timings, bootstrap CSRs and sampled containerd disk waits</summary><pre id="diagnostics"></pre></details>
<details><summary>Capture metadata and event counts</summary><pre id="metadata"></pre></details></section>
</main><div id="timeline-tooltip" class="timeline-tooltip" role="tooltip" hidden></div>
<script id="payload" type="application/json">__PAYLOAD__</script>
<script>
"use strict";
const data=JSON.parse(document.getElementById("payload").textContent), $=id=>document.getElementById(id);
let current=0, sortKey="cpu_seconds", ascending=false, hitboxes=[], popupPinned=false, rangeDrag=null, suppressTimelineClick=false;
const hiddenServices=new Set();
const ROW_HEIGHT=25, AXIS_HEIGHT=28;
const fmt=x=>Number.isFinite(x)?x.toFixed(3):"unknown";
const cell=(row,text)=>{const td=document.createElement("td");td.textContent=text;row.append(td);};
const row=(body,values)=>{const tr=document.createElement("tr");values.forEach(x=>cell(tr,x));$(body).append(tr);return tr;};
const limit=r=>r.ready_seconds??Math.max(r.trace_last??1,...r.services.map(s=>s.end??s.active_end??s.start??0),1);
function visibleThreads(r){const f=$("filter").value.toLowerCase();return r.threads.filter(t=>(t.comm+" "+t.tid).toLowerCase().includes(f));}
function serviceUnits(r){return r.services.filter(s=>s.kind==="unit");}
function visibleServices(r){return r.services.filter(s=>s.kind!=="unit"||!hiddenServices.has(s.name));}
function overlapsRange(start,end,from,to){return Number.isFinite(start)&&Number.isFinite(end)&&end>start&&end>=from&&start<=to;}
function serviceHasDataInRange(s,from,to){
 if(s.kind==="unit")return overlapsRange(s.active_start,s.active_end,from,to)
  ||[s.start,s.active_start,s.exec_exit,s.inactive].some(time=>Number.isFinite(time)&&time>=from&&time<=to);
 return overlapsRange(s.start,s.end,from,to);
}
function threadHasDataInRange(t,from,to){
 return t.cpu_bins.some(([start,cpu])=>Number.isFinite(start)&&Number.isFinite(cpu)&&cpu>0&&start+0.1>=from&&start<=to);
}
function renderServiceFilter(){
 const r=data.captures[current],units=serviceUnits(r),filter=$("service-filter-search").value.toLowerCase();
 const names=[...new Set(units.map(s=>s.name))].sort((a,b)=>a.localeCompare(b));
 const visible=names.filter(name=>!hiddenServices.has(name)).length;
 $("service-filter-summary").textContent=`Systemd service visibility · ${visible}/${names.length} shown`;
 const list=$("service-filter-list");list.replaceChildren();
 const matching=names.filter(name=>name.toLowerCase().includes(filter));
 if(!matching.length){const empty=document.createElement("span");empty.className="muted";empty.textContent="No matching systemd services in this capture.";list.append(empty);}
 for(const name of matching){
  const label=document.createElement("label");label.className="service-filter-option";
  const checkbox=document.createElement("input");checkbox.type="checkbox";checkbox.checked=!hiddenServices.has(name);
  checkbox.onchange=()=>{if(checkbox.checked)hiddenServices.delete(name);else hiddenServices.add(name);refresh();};
  const text=document.createElement("span");text.textContent=name;label.append(checkbox,text);list.append(label);
 }
}
function hidePopup(){popupPinned=false;$("timeline-tooltip").hidden=true;$("timeline").style.cursor="";}
function showPopup(text,x,y,pin=false){
 const popup=$("timeline-tooltip");popup.textContent=text;popup.hidden=false;popupPinned=pin;popup.style.pointerEvents=pin?"auto":"none";
 const rect=popup.getBoundingClientRect();
 popup.style.left=Math.max(8,Math.min(x+14,window.innerWidth-rect.width-8))+"px";
 popup.style.top=Math.max(8,Math.min(y+14,window.innerHeight-rect.height-8))+"px";
}
function hitDetail(event){
 const rect=$("timeline").getBoundingClientRect(),x=event.clientX-rect.left,y=event.clientY-rect.top;
 const hits=hitboxes.filter(b=>x>=b.x&&x<=b.x+b.w&&y>=b.y&&y<=b.y+b.h);
 const milestones=hits.filter(b=>b.milestone);
 return milestones.length?milestones.map(b=>b.detail).join("\n\n"):hits[0]?.detail;
}
function timeAtPointer(event){
 const from=Number($("from").value),to=Number($("to").value),rect=$("timeline").getBoundingClientRect();
 const left=Math.min(240,rect.width*0.35),right=rect.width-20;
 if(!Number.isFinite(from)||!Number.isFinite(to)||to<=from||right<=left)return null;
 const px=Math.max(left,Math.min(right,event.clientX-rect.left));
 return from+(px-left)/(right-left)*(to-from);
}
function updateSelectionOverlay(){
 const overlay=$("timeline-selection");
 if(!rangeDrag||!rangeDrag.moved){overlay.style.display="none";return;}
 const from=Number($("from").value),to=Number($("to").value),rect=$("timeline").getBoundingClientRect();
 const left=Math.min(240,rect.width*0.35),right=rect.width-20;
 if(!Number.isFinite(from)||!Number.isFinite(to)||to<=from||right<=left){overlay.style.display="none";return;}
 const project=time=>left+(time-from)/(to-from)*(right-left);
 const a=project(rangeDrag.startTime),b=project(rangeDrag.currentTime);
 overlay.style.left=Math.min(a,b)+"px";overlay.style.width=Math.max(1,Math.abs(b-a))+"px";overlay.style.display="block";
}
function beginRangeSelection(event){
 if(event.isPrimary===false||(event.pointerType==="mouse"&&event.button!==0))return;
 const time=timeAtPointer(event);if(time===null)return;
 rangeDrag={pointerId:event.pointerId,target:event.currentTarget.id,startX:event.clientX,startTime:time,currentTime:time,moved:false};
 event.currentTarget.setPointerCapture(event.pointerId);hidePopup();updateSelectionOverlay();
}
function moveRangeSelection(event){
 if(!rangeDrag||event.pointerId!==rangeDrag.pointerId)return false;
 const time=timeAtPointer(event);if(time===null)return true;
 rangeDrag.currentTime=time;
 if(Math.abs(event.clientX-rangeDrag.startX)>=4)rangeDrag.moved=true;
 if(rangeDrag.moved){hidePopup();updateSelectionOverlay();}
 return true;
}
function finishRangeSelection(event){
 if(!rangeDrag||event.pointerId!==rangeDrag.pointerId)return;
 moveRangeSelection(event);
 const selection=rangeDrag;rangeDrag=null;$("timeline-selection").style.display="none";
 if(event.currentTarget.hasPointerCapture(event.pointerId))event.currentTarget.releasePointerCapture(event.pointerId);
 if(!selection.moved)return;
 const lower=Number($("from").value),upper=Number($("to").value);
 let from=Math.max(lower,Math.min(selection.startTime,selection.currentTime));
 let to=Math.min(upper,Math.max(selection.startTime,selection.currentTime));
 const minimumSpan=Math.min(0.1,upper-lower);
 if(to-from<minimumSpan){
  const middle=(from+to)/2;from=middle-minimumSpan/2;to=middle+minimumSpan/2;
  if(from<lower){to+=lower-from;from=lower;}
  if(to>upper){from-=to-upper;to=upper;}
 }
 $("from").value=from.toFixed(3);$("to").value=to.toFixed(3);$("timeline-scroll").scrollTop=0;
 suppressTimelineClick=selection.target==="timeline";
 if(suppressTimelineClick)setTimeout(()=>{suppressTimelineClick=false;},0);
 refresh();
}
function cancelRangeSelection(event){
 if(!rangeDrag||event.pointerId!==rangeDrag.pointerId)return;
 rangeDrag=null;$("timeline-selection").style.display="none";
}
function cards(){
 $("cards").replaceChildren();
 data.captures.forEach((r,i)=>{
  for(const [label,value] of [[`${data.labels[i]} · boot → Ready`,fmt(r.ready_seconds)+" s"],
   ["First trace event",fmt(r.trace_first)+" s"],["Observed block events",String(r.block_event_count)]]){
    const box=document.createElement("div");box.className="card";
    const labelNode=document.createElement("span");labelNode.className="muted";labelNode.textContent=label;
    const valueNode=document.createElement("strong");valueNode.textContent=value;box.append(labelNode,valueNode);$("cards").append(box);
  }
 });
}
function draw(){
 const r=data.captures[current],canvas=$("timeline"),ctx=canvas.getContext("2d");
 const axis=$("timeline-axis"),axisCtx=axis.getContext("2d"),scroll=$("timeline-scroll");
 hidePopup();hitboxes=[];$("detail").textContent="";
 const from=Number($("from").value),to=Number($("to").value);
 const validRange=Number.isFinite(from)&&Number.isFinite(to)&&to>from;
 const services=visibleServices(r).filter(s=>!validRange||serviceHasDataInRange(s,from,to));
 const threads=visibleThreads(r).filter(t=>!validRange||threadHasDataInRange(t,from,to)).slice().sort((a,b)=>(b.cpu_seconds??0)-(a.cpu_seconds??0));
 const rowCount=services.length+3+threads.length;
 const width=Math.max(300,scroll.clientWidth),height=scroll.clientHeight;
 const contentHeight=Math.max(height,rowCount*ROW_HEIGHT);
 $("timeline-spacer").style.height=contentHeight+"px";$("timeline-sticky").style.height=height+"px";
 const maxScroll=Math.max(0,contentHeight-height);
 if(scroll.scrollTop>maxScroll)scroll.scrollTop=maxScroll;
 const scrollTop=scroll.scrollTop,left=Math.min(240,width*0.35);
 const ratio=window.devicePixelRatio||1;
 axis.width=width*ratio;axis.height=AXIS_HEIGHT*ratio;axis.style.width=width+"px";axis.style.height=AXIS_HEIGHT+"px";axisCtx.scale(ratio,ratio);
 canvas.width=width*ratio;canvas.height=height*ratio;canvas.style.width=width+"px";canvas.style.height=height+"px";ctx.scale(ratio,ratio);
 ctx.clearRect(0,0,width,height);axisCtx.clearRect(0,0,width,AXIS_HEIGHT);hitboxes=[];
 const labels=$("timeline-service-labels");labels.replaceChildren();labels.style.width=(left-8)+"px";labels.style.height=height+"px";
 if(!Number.isFinite(from)||!Number.isFinite(to)||to<=from){$("milestone-legend").replaceChildren();$("detail").textContent="Choose a finite interval with To > From.";updateSelectionOverlay();return;}
 const x=t=>left+(t-from)/(to-from)*(width-left-20);
 axisCtx.font="12px system-ui";axisCtx.fillStyle="#9bb0cb";axisCtx.strokeStyle="#30415b";
 ctx.strokeStyle="#30415b";
 for(let i=0;i<=6;i++){
  const t=from+(to-from)*i/6,px=x(t);axisCtx.fillText(t.toFixed(2)+"s",px,16);
  axisCtx.beginPath();axisCtx.moveTo(px,AXIS_HEIGHT-4);axisCtx.lineTo(px,AXIS_HEIGHT);axisCtx.stroke();
  ctx.beginPath();ctx.moveTo(px,0);ctx.lineTo(px,height);ctx.stroke();
 }
 function addRowLabel(text,rowY){
  const label=document.createElement("span");label.className="timeline-row-label";
  label.textContent=text;label.title=text;label.style.top=(rowY-10)+"px";
  label.style.width=(left-12)+"px";labels.append(label);
 }
 let y=0;
 function bar(label,start,end,color,alpha,detail){
  if(!Number.isFinite(start)||!Number.isFinite(end)||end<from||start>to)return;
  const a=x(Math.max(start,from)),b=x(Math.min(end,to));ctx.globalAlpha=alpha;ctx.fillStyle=color;
  ctx.fillRect(a,y-10,Math.max(1,b-a),14);ctx.globalAlpha=1;
  hitboxes.push({x:a,y:y-12,w:Math.max(5,b-a),h:20,detail});
 }
 function marker(time,color,offset,detail){
  if(!Number.isFinite(time)||time<from||time>to)return;
  const px=x(time);ctx.strokeStyle=color;ctx.beginPath();ctx.moveTo(px,y-11+offset);ctx.lineTo(px,y+5+offset);ctx.stroke();
  hitboxes.push({x:px-4,y:y-13+offset,w:8,h:20,detail});
 }
 const firstRow=Math.max(0,Math.floor(scrollTop/ROW_HEIGHT));
 const lastRow=Math.min(rowCount,Math.ceil((scrollTop+height)/ROW_HEIGHT));
 for(let index=firstRow;index<lastRow;index++){
  y=index*ROW_HEIGHT-scrollTop+ROW_HEIGHT/2;
  if(index<services.length){
   const s=services[index];addRowLabel(s.name,y);
   if(s.kind==="unit"){
    const state=`${s.active_state??"unknown"}${s.sub_state?"/"+s.sub_state:""}`;
    if(Number.isFinite(s.active_start)&&Number.isFinite(s.active_end)&&s.active_end>s.active_start){
     const endDescription=s.active_at_cutoff?(r.ready_seconds!==null?"still active at Ready":"active at trace end"):"unit became inactive";
     bar(s.name,s.active_start,s.active_end,"#59d9dc",0.85,
      `${s.name}: active ${fmt(s.active_start)}–${fmt(s.active_end)}s (${endDescription}); state at collection ${state}; ${s.source}`);
    }
    marker(s.start,"#9bb0cb",-6,`${s.name}: process start at ${fmt(s.start)}s; state ${state}; ${s.source}`);
    marker(s.active_start,"#59d9dc",-2,`${s.name}: entered active state at ${fmt(s.active_start)}s; ${s.source}`);
    marker(s.exec_exit,"#b99cff",3,`${s.name}: process exit at ${fmt(s.exec_exit)}s; ${s.source}`);
    marker(s.inactive,"#f38ba8",7,`${s.name}: entered inactive state at ${fmt(s.inactive)}s; ${s.source}`);
   }else{
    bar(s.name,s.start,s.end,"#59d9dc",0.85,
     `${s.name}: task ${fmt(s.start)}–${fmt(s.end)}s; ${s.source}`);
   }
  }else if(index===services.length){
   addRowLabel("Block I/O events (not latency)",y);
   for(const [start,counts] of r.io_bins??[]){bar("I/O",start,start+0.1,"#efb668",0.8,`Block events at ${fmt(start)}s: ${JSON.stringify(counts)}; no thread attribution`);}
  }else if(index===services.length+1||index===services.length+2){
   const direction=index===services.length+1?"tx":"rx",color=direction==="tx"?"#a6e3a1":"#89b4fa";
   const label=`Network ${direction.toUpperCase()}`,covered=r.network_coverage?.[direction];
   addRowLabel(label+(covered?" events":" (not captured)"),y);
   for(const [start,counts] of r.network_bins??[]){
    const matching=Object.fromEntries(Object.entries(counts).filter(([name])=>r.network_event_directions?.[name]===direction));
    if(Object.keys(matching).length)bar(label,start,start+0.1,color,0.8,`${label} events at ${fmt(start)}s: ${JSON.stringify(matching)}; device activity, not packet counts, delivery or network wait`);
   }
  }else{
   const t=threads[index-services.length-3];addRowLabel(`${t.comm} [${t.tid}/${t.generation}]`,y);
   for(const [start,cpu] of t.cpu_bins)bar(t.comm,start,start+0.1,"#b99cff",Math.max(0.25,Math.min(1,cpu/0.1)),`${t.comm} [${t.tid}]: ${fmt(cpu)} CPU seconds in bin ${fmt(start)}s`);
  }
 }
 ctx.setLineDash([4,4]);
 const legend=document.createDocumentFragment();
 const colors=["#9bb0cb","#59d9dc","#b99cff","#f38ba8","#a6e3a1","#89b4fa"];
 const milestones=r.milestones.map((m,i)=>({...m,color:m.name.includes("Ready")?"#efb668":colors[i%colors.length]})).sort((a,b)=>a.t-b.t);
 for(const m of milestones){if(m.t<from||m.t>to)continue;
  const detail=`${m.name} @ ${fmt(m.t)}s\nEvidence: ${m.source}`;
  const button=document.createElement("button"),swatch=document.createElement("span"),label=document.createElement("span");
  button.type="button";button.setAttribute("aria-describedby","timeline-tooltip");swatch.className="milestone-swatch";swatch.style.borderColor=m.color;swatch.setAttribute("aria-hidden","true");
  label.textContent=`${m.name} · ${fmt(m.t)} s`;button.append(swatch,label);legend.append(button);
  const show=pin=>{const box=button.getBoundingClientRect();showPopup(detail,box.left,box.bottom,pin);};
  button.onpointerenter=()=>{if(!popupPinned)show(false);};button.onfocus=()=>{if(!popupPinned)show(false);};
  button.onpointerleave=button.onblur=()=>{if(!popupPinned)hidePopup();};
  button.onclick=()=>{show(true);$("detail").textContent=detail;};
  ctx.strokeStyle=m.color;ctx.beginPath();ctx.moveTo(x(m.t),0);ctx.lineTo(x(m.t),height);ctx.stroke();
  hitboxes.push({x:x(m.t)-4,y:0,w:8,h:height,detail,milestone:true});
 }
 $("milestone-legend").replaceChildren(legend);
 ctx.setLineDash([]);
 updateSelectionOverlay();
}
function refresh(){
 const r=data.captures[current];$("threads").replaceChildren();
 const threads=visibleThreads(r).slice().sort((a,b)=>{
  const av=a[sortKey],bv=b[sortKey],v=typeof av==="string"?av.localeCompare(bv):av-bv;return ascending?v:-v;
 });
 for(const t of threads.slice(0,2000))row("threads",[t.comm,`${t.tid}/${t.generation}`,fmt(t.observed_span_seconds),fmt(t.cpu_seconds),fmt(t.runnable_seconds),t.switches]);
 $("services").replaceChildren();
 for(const s of r.services){
  if(s.kind==="unit"){
   const active=s.active_start==null?"—":fmt(s.active_start);
   const inactive=s.inactive==null?"—":fmt(s.inactive);
   const exit=s.exec_exit==null?"—":fmt(s.exec_exit);
   row("services",[s.name,`${s.active_state??"unknown"}${s.sub_state?"/"+s.sub_state:""}`,
    fmt(s.start),active,`inactive ${inactive} · exit ${exit}`,s.source]);
  }else{
   row("services",[s.name,"init-aks-cloud task",fmt(s.start),"—",fmt(s.end),s.source]);
  }
 }
 r.milestones.forEach(m=>row("services",[m.name,"milestone",fmt(m.t),"—","—",m.source]));
 $("warnings").replaceChildren();
 for(const text of r.warnings){const li=document.createElement("li");li.textContent=text;$("warnings").append(li);}
 $("diagnostics").textContent=JSON.stringify(r.targeted_diagnostics??{},null,2);
 $("metadata").textContent=JSON.stringify({metadata:r.metadata,event_counts:r.event_counts,trace_first:r.trace_first,trace_last:r.trace_last,clock_sample_uncertainty_seconds:r.clock_sample_uncertainty_seconds},null,2);
 renderServiceFilter();
 draw();
}
for(const [label,key] of [["Thread","comm"],["TID/generation","tid"],["Observed span (s)","observed_span_seconds"],["CPU (s)","cpu_seconds"],["Runnable wait (s)","runnable_seconds"],["Switch-outs","switches"]]){
 const th=document.createElement("th"),button=document.createElement("button");button.textContent=label;button.onclick=()=>{ascending=sortKey===key?!ascending:false;sortKey=key;refresh();};th.append(button);$("thread-head").append(th);
}
data.captures.forEach((r,i)=>{const option=document.createElement("option");option.value=i;option.textContent=data.labels[i]+" · "+r.metadata.backend;$("capture").append(option);
 const badge=document.createElement("span");badge.className="badge";badge.textContent=r.metadata.backend+" / "+r.metadata.kernel+" / "+r.metadata.boot_id;$("identity").append(badge);
});
function reset(){const r=data.captures[current];rangeDrag=null;$("timeline-selection").style.display="none";$("from").value=0;$("to").value=(Math.ceil(limit(r)*1000)/1000).toFixed(3);$("timeline-scroll").scrollTop=0;refresh();}
$("capture").onchange=()=>{current=Number($("capture").value);reset();};
["filter","from","to"].forEach(id=>$(id).addEventListener("input",refresh));$("reset").onclick=reset;
$("service-filter-search").addEventListener("input",renderServiceFilter);
$("show-services").onclick=()=>{hiddenServices.clear();refresh();};
$("hide-services").onclick=()=>{for(const capture of data.captures)for(const s of serviceUnits(capture))hiddenServices.add(s.name);refresh();};
for(const element of [$("timeline-axis"),$("timeline")]){
 element.addEventListener("pointerdown",beginRangeSelection);
 element.addEventListener("pointerup",finishRangeSelection);
 element.addEventListener("pointercancel",cancelRangeSelection);
}
$("timeline-axis").addEventListener("pointermove",moveRangeSelection);
$("timeline").onpointermove=event=>{
 if(moveRangeSelection(event))return;
 if(popupPinned)return;
 const detail=hitDetail(event);if(detail){showPopup(detail,event.clientX,event.clientY);$("timeline").style.cursor="pointer";}else hidePopup();
};
$("timeline").onpointerleave=()=>{if(!popupPinned)hidePopup();};
$("timeline").onclick=event=>{
 if(suppressTimelineClick){suppressTimelineClick=false;event.preventDefault();return;}
 const detail=hitDetail(event);if(detail){showPopup(detail,event.clientX,event.clientY,true);$("detail").textContent=detail;}else hidePopup();
};
$("timeline-scroll").addEventListener("scroll",draw,{passive:true});
document.addEventListener("click",event=>{if(!event.target.closest("#timeline, #milestone-legend, #timeline-tooltip"))hidePopup();});
document.addEventListener("keydown",event=>{if(event.key==="Escape")hidePopup();});
window.addEventListener("scroll",event=>{if(!$("timeline-tooltip").contains(event.target))hidePopup();},true);
const resizeHandle=$("timeline-resize"),timelineScroll=$("timeline-scroll"),MIN_TIMELINE_HEIGHT=220;
let resizeDrag=null;
function maxTimelineHeight(){return Math.max(MIN_TIMELINE_HEIGHT,window.innerHeight-100);}
function updateResizeAria(){
 resizeHandle.setAttribute("aria-valuemax",String(Math.round(maxTimelineHeight())));
 resizeHandle.setAttribute("aria-valuenow",String(Math.round(timelineScroll.clientHeight)));
}
function setTimelineHeight(height){
 const next=Math.max(MIN_TIMELINE_HEIGHT,Math.min(maxTimelineHeight(),height));
 timelineScroll.style.height=next+"px";updateResizeAria();draw();
}
resizeHandle.addEventListener("pointerdown",event=>{
 if(event.pointerType==="mouse"&&event.button!==0)return;
 event.preventDefault();resizeDrag={pointerId:event.pointerId,startY:event.clientY,startHeight:timelineScroll.clientHeight};
 resizeHandle.setPointerCapture(event.pointerId);document.body.style.cursor="ns-resize";document.body.style.userSelect="none";
});
resizeHandle.addEventListener("pointermove",event=>{
 if(resizeDrag&&event.pointerId===resizeDrag.pointerId)setTimelineHeight(resizeDrag.startHeight+event.clientY-resizeDrag.startY);
});
function finishResize(event){
 if(!resizeDrag||event.pointerId!==resizeDrag.pointerId)return;
 if(resizeHandle.hasPointerCapture(event.pointerId))resizeHandle.releasePointerCapture(event.pointerId);
 resizeDrag=null;document.body.style.cursor="";document.body.style.userSelect="";
}
resizeHandle.addEventListener("pointerup",finishResize);resizeHandle.addEventListener("pointercancel",finishResize);
resizeHandle.addEventListener("keydown",event=>{
 let height=timelineScroll.clientHeight;
 if(event.key==="ArrowDown")height+=40;else if(event.key==="ArrowUp")height-=40;
 else if(event.key==="Home")height=MIN_TIMELINE_HEIGHT;else if(event.key==="End")height=maxTimelineHeight();else return;
 event.preventDefault();setTimelineHeight(height);
});
updateResizeAria();
window.addEventListener("resize",()=>{if(timelineScroll.style.height)setTimelineHeight(timelineScroll.clientHeight);else{updateResizeAria();draw();}});
if(data.captures.length>=2){
 $("comparison").hidden=false;
 if(data.captures.length!==2){
  $("pair-comparison").hidden=true;$("repeat-summary").hidden=false;
  const grouped=new Map();
  data.captures.forEach((capture,index)=>{
   const backend=capture.metadata.backend,values=grouped.get(backend)??[];
   row("trial-readiness",[data.labels[index],backend,fmt(capture.ready_seconds)]);
   values.push(capture.ready_seconds);grouped.set(backend,values);
  });
  for(const [backend,values] of grouped){
   const sorted=values.filter(Number.isFinite).sort((a,b)=>a-b);
   const median=sorted.length%2?sorted[(sorted.length-1)/2]:(sorted[sorted.length/2-1]+sorted[sorted.length/2])/2;
   row("trial-summary",[backend,String(sorted.length),fmt(sorted[0]),fmt(median),fmt(sorted[sorted.length-1])]);
  }
  const patterns=new Map();
  data.captures.forEach((capture,index)=>{
   if(capture.metadata.backend!=="ftrace")return;
   for(const [event,count] of Object.entries(capture.event_counts??{})){
    if(count>0){const samples=patterns.get(`event · ${event}`)??new Map();samples.set(index,count);patterns.set(`event · ${event}`,samples);}
   }
   const serviceTimes=new Map();
   for(const service of capture.services){
    const start=service.kind==="unit"?service.active_start:service.start;
    const end=service.kind==="unit"?service.active_end:service.end;
    if(Number.isFinite(start)&&Number.isFinite(end)&&end>start){
     const key=`${service.kind==="unit"?"service":"task"} · ${service.name}`;
     serviceTimes.set(key,(serviceTimes.get(key)??0)+end-start);
    }
   }
   for(const [name,duration] of serviceTimes){
    const key=name,samples=patterns.get(key)??new Map();
    samples.set(index,duration);patterns.set(key,samples);
   }
  });
  const recurring=[...patterns].filter(([,samples])=>samples.size>=2)
   .sort((a,b)=>b[1].size-a[1].size||a[0].localeCompare(b[0]));
  for(const [name,samples] of recurring){
   const values=[...samples.values()].sort((a,b)=>a-b);
   const median=values.length%2?values[(values.length-1)/2]:(values[values.length/2-1]+values[values.length/2])/2;
   const suffix=name.startsWith("event · ")?" events":" s";
   row("repeat-patterns",[name,`${values.length}/${grouped.get("ftrace").length}`,fmt(values[0])+suffix,fmt(median)+suffix,fmt(values[values.length-1])+suffix]);
  }
 }else{
 const [a,b]=data.captures;
 const fields=["backend","kernel","architecture","image_label"],notes=[];
 for(const field of fields)if(a.metadata[field]!==b.metadata[field])notes.push(`${field} differs: ${a.metadata[field]} / ${b.metadata[field]}`);
 for(const field of ["vm_size","node_image_version","network_plugin","network_policy","network_dataplane","os_sku","kubernetes_version","canonical_base_version","cluster_resource_id"]){
  const av=a.readiness?.configuration?.[field],bv=b.readiness?.configuration?.[field];
  if(av==null||bv==null||av===""||bv==="")notes.push(`${field}: comparability unknown`);else if(av!==bv)notes.push(`${field} differs: ${av} / ${bv}`);
 }
 $("compatibility").textContent=notes.length?notes.join("; "):"Recorded configuration fields match. External load and run-to-run variance still need review.";
 function delta(name,av,bv){row("deltas",[name,fmt(av),fmt(bv),fmt(Number.isFinite(av)&&Number.isFinite(bv)?bv-av:null)]);}
 delta("Linux boot → Ready",a.ready_seconds,b.ready_seconds);
 function serviceDurations(capture){
  return new Map(capture.services.flatMap(s=>{
   const start=s.kind==="unit"?s.active_start:s.start,end=s.kind==="unit"?s.active_end:s.end;
   return Number.isFinite(start)&&Number.isFinite(end)&&end>start?[[s.name,end-start]]:[];
  }));
 }
 const am=serviceDurations(a),bm=serviceDurations(b);
 for(const name of new Set([...am.keys(),...bm.keys()]))delta(name+" active interval",am.get(name),bm.get(name));
 function group(r){const result=new Map();for(const t of r.threads){const v=result.get(t.comm)??{cpu:0,wait:0};v.cpu=Number.isFinite(v.cpu)&&Number.isFinite(t.cpu_seconds)?v.cpu+t.cpu_seconds:null;v.wait=Number.isFinite(v.wait)&&Number.isFinite(t.runnable_seconds)?v.wait+t.runnable_seconds:null;result.set(t.comm,v);}return result;}
 const ag=group(a),bg=group(b),names=[...new Set([...ag.keys(),...bg.keys()])];
 names.sort((x,y)=>(ag.get(y)?.cpu??0)+(bg.get(y)?.cpu??0)-(ag.get(x)?.cpu??0)-(bg.get(x)?.cpu??0));
 for(const name of names.slice(0,50)){
  delta(`CPU sum · threads named ${name}`,ag.get(name)?.cpu,bg.get(name)?.cpu);
  delta(`Runnable sum · threads named ${name}`,ag.get(name)?.wait,bg.get(name)?.wait);
 }
 }
}
cards();reset();
</script></body></html>
"""


def write_html(path, captures, labels):
    payload = json.dumps(
        {"captures": captures, "labels": labels}, ensure_ascii=True, allow_nan=False,
    ).replace("<", "\\u003c").replace(">", "\\u003e").replace("&", "\\u0026")
    path.write_text(PAGE.replace("__PAYLOAD__", payload), encoding="utf-8")
