package server

import "net/http"

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(dashHTML))
}

const dashHTML = `<!DOCTYPE html>
<html lang="en"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Granary</title>
<style>
:root{--bg:#1a1410;--bg2:#241e18;--bg3:#2e261e;--rust:#c45d2c;--rl:#e8753a;--leather:#a0845c;--ll:#c4a87a;--cream:#f0e6d3;--cd:#bfb5a3;--cm:#7a7060;--gold:#d4a843;--green:#4a9e5c;--red:#c44040;--mono:'JetBrains Mono',Consolas,monospace;--serif:'Libre Baskerville',Georgia,serif}
*{margin:0;padding:0;box-sizing:border-box}body{background:var(--bg);color:var(--cream);font-family:var(--mono);font-size:13px;line-height:1.6}
.hdr{padding:.6rem 1.2rem;border-bottom:1px solid var(--bg3);display:flex;justify-content:space-between;align-items:center}
.hdr h1{font-family:var(--serif);font-size:1rem}.hdr h1 span{color:var(--rl)}
.main{max-width:800px;margin:0 auto;padding:1rem 1.2rem}
.btn{font-family:var(--mono);font-size:.68rem;padding:.3rem .6rem;border:1px solid;cursor:pointer;background:transparent;transition:.15s;white-space:nowrap}
.btn-p{border-color:var(--rust);color:var(--rl)}.btn-p:hover{background:var(--rust);color:var(--cream)}
.btn-d{border-color:var(--bg3);color:var(--cm)}.btn-d:hover{border-color:var(--red);color:var(--red)}
.overview{display:flex;gap:1.5rem;margin-bottom:1rem;font-size:.7rem;color:var(--leather)}
.overview .stat b{display:block;font-size:1.2rem;color:var(--cream)}
.bucket-card{background:var(--bg2);border:1px solid var(--bg3);padding:.6rem;margin-bottom:.4rem;cursor:pointer;transition:.1s}
.bucket-card:hover{background:var(--bg3)}
.bucket-card h3{font-size:.8rem;margin-bottom:.15rem;display:flex;align-items:center;gap:.4rem}
.bucket-meta{font-size:.65rem;color:var(--cm);display:flex;gap:.7rem}
.pub-badge{font-size:.55rem;padding:0 .25rem;border-radius:2px;color:var(--green);border:1px solid var(--green)}
.prv-badge{font-size:.55rem;padding:0 .25rem;border-radius:2px;color:var(--cm);border:1px solid var(--bg3)}
.obj-row{display:flex;align-items:center;gap:.5rem;padding:.35rem .5rem;border-bottom:1px solid var(--bg3);font-size:.72rem}
.obj-key{flex:1;color:var(--rl);cursor:pointer}.obj-key:hover{color:var(--gold)}
.obj-size{color:var(--cm);font-size:.65rem;width:60px;text-align:right}
.obj-type{color:var(--leather);font-size:.6rem}
.empty{text-align:center;padding:2rem;color:var(--cm);font-style:italic;font-family:var(--serif)}
.upload-zone{border:2px dashed var(--bg3);padding:1rem;text-align:center;margin-bottom:.5rem;cursor:pointer;transition:.2s;color:var(--cm);font-size:.75rem}
.upload-zone:hover{border-color:var(--rust);color:var(--rl)}
.modal-bg{position:fixed;top:0;left:0;right:0;bottom:0;background:rgba(0,0,0,.65);display:flex;align-items:center;justify-content:center;z-index:100}
.modal{background:var(--bg2);border:1px solid var(--bg3);padding:1.5rem;width:90%;max-width:450px}
.modal h2{font-family:var(--serif);font-size:.9rem;margin-bottom:1rem}
label.fl{display:block;font-size:.65rem;color:var(--leather);text-transform:uppercase;letter-spacing:1px;margin-bottom:.2rem;margin-top:.5rem}
input[type=text],input[type=file]{background:var(--bg);border:1px solid var(--bg3);color:var(--cream);padding:.35rem .5rem;font-family:var(--mono);font-size:.78rem;width:100%;outline:none}
</style>
<link href="https://fonts.googleapis.com/css2?family=Libre+Baskerville:ital@0;1&family=JetBrains+Mono:wght@400;600&display=swap" rel="stylesheet">
</head><body>
<div class="hdr"><h1><span>Granary</span></h1><button class="btn btn-p" onclick="showNewBucket()">+ Bucket</button></div>
<div class="main"><div id="upgrade-banner" style="display:none;background:#241e18;border:1px solid #8b3d1a;border-left:3px solid #c45d2c;padding:.6rem 1rem;font-size:.78rem;color:#bfb5a3;margin-bottom:.8rem"><strong style="color:#f0e6d3">Free tier</strong> — 10 items max. <a href="https://stockyard.dev/granary/" target="_blank" style="color:#e8753a">Upgrade to Pro →</a></div>
<div class="overview" id="overview"></div>
<div id="bucketList"></div>
<div id="detail" style="display:none;margin-top:1rem"></div>
</div>
<div id="modal"></div>

<script>
let buckets=[],curBucket='';
async function api(u,o){return(await fetch(u,o)).json()}
function esc(s){return String(s||'').replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;')}
function fmtSize(b){if(b<1024)return b+' B';if(b<1024*1024)return(b/1024).toFixed(1)+' KB';if(b<1024*1024*1024)return(b/(1024*1024)).toFixed(1)+' MB';return(b/(1024*1024*1024)).toFixed(1)+' GB'}

async function init(){
  const[bd,sd]=await Promise.all([api('/api/buckets'),api('/api/stats')]);
  buckets=bd.buckets||[];
  document.getElementById('overview').innerHTML=
    '<div class="stat"><b>'+sd.buckets+'</b>Buckets</div>'+
    '<div class="stat"><b>'+sd.objects+'</b>Objects</div>'+
    '<div class="stat"><b>'+fmtSize(sd.total_bytes)+'</b>Total Size</div>';
  document.getElementById('bucketList').innerHTML=buckets.length?buckets.map(b=>
    '<div class="bucket-card" onclick="openBucket(\''+b.id+'\')"><h3>'+(b.public?'<span class="pub-badge">public</span>':'<span class="prv-badge">private</span>')+' '+esc(b.name)+'</h3>'+
    '<div class="bucket-meta"><span>'+b.object_count+' objects</span><span>'+fmtSize(b.total_size)+'</span></div></div>'
  ).join(''):'<div class="empty">No buckets yet.</div>'
  if(curBucket)openBucket(curBucket)
}

async function openBucket(id){
  curBucket=id;
  const[b,od]=await Promise.all([api('/api/buckets/'+id),api('/api/buckets/'+id+'/objects')]);
  const objects=od.objects||[];
  const publicUrl=b.public?'<div style="font-size:.65rem;color:var(--cm);margin-bottom:.5rem">Public URL: <span style="color:var(--rl)">/f/'+esc(b.name)+'/</span></div>':'';
  document.getElementById('detail').style.display='block';
  document.getElementById('detail').innerHTML=
    '<div style="display:flex;justify-content:space-between;margin-bottom:.5rem"><span style="font-size:.75rem;color:var(--leather)">'+esc(b.name)+' ('+objects.length+' objects, '+fmtSize(b.total_size)+')</span>'+
    '<div style="display:flex;gap:.3rem"><button class="btn btn-p" onclick="showUpload()">Upload</button><button class="btn btn-d" onclick="if(confirm(\'Delete bucket?\'))delBucket(\''+id+'\')">Del</button></div></div>'+
    publicUrl+
    (objects.length?objects.map(o=>
      '<div class="obj-row"><a class="obj-key" href="/api/buckets/'+id+'/objects/'+encodeURIComponent(o.key)+'" target="_blank">'+esc(o.key)+'</a>'+
      '<span class="obj-type">'+esc(o.content_type)+'</span>'+
      '<span class="obj-size">'+fmtSize(o.size)+'</span>'+
      '<span style="cursor:pointer;font-size:.55rem;color:var(--cm)" onclick="delObj(\''+esc(o.key)+'\')">del</span></div>'
    ).join(''):'<div class="empty" style="padding:1rem">Empty bucket. Upload some files.</div>')
}

async function delBucket(id){await api('/api/buckets/'+id,{method:'DELETE'});curBucket='';document.getElementById('detail').style.display='none';init()}
async function delObj(key){await api('/api/buckets/'+curBucket+'/objects/'+encodeURIComponent(key),{method:'DELETE'});openBucket(curBucket);init()}

function showUpload(){
  document.getElementById('modal').innerHTML='<div class="modal-bg" onclick="if(event.target===this)closeModal()"><div class="modal"><h2>Upload File</h2>'+
    '<label class="fl">File</label><input type="file" id="uf-file">'+
    '<label class="fl">Key (optional, defaults to filename)</label><input type="text" id="uf-key">'+
    '<div style="display:flex;gap:.5rem;margin-top:1rem"><button class="btn btn-p" onclick="doUpload()">Upload</button><button class="btn btn-d" onclick="closeModal()">Cancel</button></div></div></div>'
}

async function doUpload(){
  const file=document.getElementById('uf-file').files[0];
  if(!file){alert('Select a file');return}
  const fd=new FormData();fd.append('file',file);
  const key=document.getElementById('uf-key').value;if(key)fd.append('key',key);
  await fetch('/api/upload/'+curBucket,{method:'POST',body:fd});
  closeModal();openBucket(curBucket);init()
}

function showNewBucket(){
  document.getElementById('modal').innerHTML='<div class="modal-bg" onclick="if(event.target===this)closeModal()"><div class="modal"><h2>New Bucket</h2>'+
    '<label class="fl">Name</label><input type="text" id="nb-name" placeholder="assets">'+
    '<label class="fl" style="display:flex;align-items:center;gap:.4rem;margin-top:.7rem"><input type="checkbox" id="nb-pub"> Public (files accessible without auth at /f/name/key)</label>'+
    '<div style="display:flex;gap:.5rem;margin-top:1rem"><button class="btn btn-p" onclick="saveBucket()">Create</button><button class="btn btn-d" onclick="closeModal()">Cancel</button></div></div></div>'
}

async function saveBucket(){
  const b={name:document.getElementById('nb-name').value,public:document.getElementById('nb-pub').checked};
  if(!b.name){alert('Name required');return}
  await api('/api/buckets',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(b)});
  closeModal();init()
}

function closeModal(){document.getElementById('modal').innerHTML=''}
init()
fetch('/api/tier').then(r=>r.json()).then(j=>{if(j.tier==='free'){var b=document.getElementById('upgrade-banner');if(b)b.style.display='block'}}).catch(()=>{var b=document.getElementById('upgrade-banner');if(b)b.style.display='block'});
</script><script>
(function(){
  fetch('/api/config').then(function(r){return r.json()}).then(function(cfg){
    if(!cfg||typeof cfg!=='object')return;
    if(cfg.dashboard_title){
      document.title=cfg.dashboard_title;
      var h1=document.querySelector('h1');
      if(h1){
        var inner=h1.innerHTML;
        var firstSpan=inner.match(/<span[^>]*>[^<]*<\/span>/);
        if(firstSpan){h1.innerHTML=firstSpan[0]+' '+cfg.dashboard_title}
        else{h1.textContent=cfg.dashboard_title}
      }
    }
  }).catch(function(){});
})();
</script>
</body></html>`
