const $ = (s) => document.querySelector(s);
const $$ = (s) => [...document.querySelectorAll(s)];
let state = {jobs: [], setupRequired: false, authenticated: false, version: '', importBundle: null, update: null};

const api = async (path, options={}) => {
  const opts = {...options, headers: {...(options.headers||{})}};
  if (opts.body && typeof opts.body !== 'string') {
    opts.headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(opts.body);
  }
  const res = await fetch(path, opts);
  let data = {};
  try { data = await res.json(); } catch (_) {}
  if (res.status === 401 && path !== '/api/login') {
    showAuth(false);
    throw new Error(data.error || 'Authentication required');
  }
  if (!res.ok) throw new Error(data.error || `Request failed (${res.status})`);
  return data;
};

function toast(message, error=false) {
  const el = $('#toast'); el.textContent = message; el.classList.toggle('error', error); el.classList.remove('hidden');
  clearTimeout(toast.timer); toast.timer = setTimeout(() => el.classList.add('hidden'), 3200);
}

async function boot() {
  try {
    const s = await api('/api/status'); state = {...state, ...s};
    $('#versionText').textContent = `v${s.version}`; const cv=$('#currentVersionText'); if(cv) cv.textContent=`v${s.version}`;
    if (s.setupRequired) showAuth(true);
    else if (!s.authenticated) showAuth(false);
    else await showApp();
  } catch (e) { showAuth(false); $('#authError').textContent = e.message; }
}

function showAuth(setup) {
  state.setupRequired = setup;
  $('#appView').classList.add('hidden'); $('#authView').classList.remove('hidden');
  $('#authTitle').textContent = setup ? 'Set up Cron Manager' : 'Welcome back';
  $('#authText').textContent = setup ? 'Create a local password for this NAS. It is stored only on the device.' : 'Sign in to manage scheduled tasks on this NAS.';
  $('#authSubmit').textContent = setup ? 'Create password' : 'Sign in';
  $('#confirmLabel').classList.toggle('hidden', !setup); $('#confirmPassword').classList.toggle('hidden', !setup);
  $('#password').autocomplete = setup ? 'new-password' : 'current-password';
  $('#password').value=''; $('#confirmPassword').value=''; $('#authError').textContent='';
}

async function showApp() {
  $('#authView').classList.add('hidden'); $('#appView').classList.remove('hidden');
  await loadJobs();
}

$('#authForm').addEventListener('submit', async (e) => {
  e.preventDefault(); $('#authError').textContent='';
  const password = $('#password').value;
  if (state.setupRequired && password !== $('#confirmPassword').value) { $('#authError').textContent='Passwords do not match.'; return; }
  try {
    await api(state.setupRequired ? '/api/setup' : '/api/login', {method:'POST', body:{password}});
    state.setupRequired=false; state.authenticated=true; await showApp();
  } catch(e) { $('#authError').textContent=e.message; }
});

async function loadJobs() {
  try {
    const data = await api('/api/jobs'); state.jobs = data.jobs || []; renderJobs();
  } catch(e) { toast(e.message,true); }
}

function renderJobs() {
  const q = $('#searchInput').value.trim().toLowerCase(); const filter = $('#filterSelect').value;
  const jobs = state.jobs.filter(j => {
    const matchText = !q || `${j.name} ${j.schedule} ${j.command} ${j.human}`.toLowerCase().includes(q);
    const matchFilter = filter==='all' || (filter==='enabled'&&j.enabled) || (filter==='disabled'&&!j.enabled) || (filter==='managed'&&j.managed) || (filter==='imported'&&!j.managed);
    return matchText && matchFilter;
  });
  $('#totalStat').textContent=state.jobs.length;
  $('#enabledStat').textContent=state.jobs.filter(j=>j.enabled).length;
  $('#managedStat').textContent=state.jobs.filter(j=>j.managed).length;
  $('#importedStat').textContent=state.jobs.filter(j=>!j.managed).length;
  $('#jobCountBadge').textContent=`${jobs.length} job${jobs.length===1?'':'s'}`;
  const list=$('#jobsList'); list.innerHTML=''; $('#emptyState').classList.toggle('hidden', jobs.length!==0);
  for(const j of jobs) list.appendChild(jobRow(j));
}

function jobRow(j) {
  const row=document.createElement('div'); row.className='job-row';
  const last=j.running ? '<strong>Running now…</strong><span>Manual execution</span>' : j.lastRun ? `<strong class="${j.lastRun.exitCode===0?'run-good':'run-bad'}">${j.lastRun.exitCode===0?'Success':'Exit '+j.lastRun.exitCode}</strong><span>${relativeTime(j.lastRun.finished)} · ${escapeHtml(j.lastRun.source)}</span>` : '<strong>Not tracked yet</strong><span>History begins after adoption/run</span>';
  row.innerHTML=`<div class="job-main"><strong>${escapeHtml(j.name)}</strong><div class="job-meta"><span class="badge ${j.managed?'managed':'imported'}">${j.managed?'Managed':'Imported'}</span><span class="badge ${j.enabled?'enabled':'disabled'}">${j.enabled?'Enabled':'Disabled'}</span></div></div><div class="schedule"><strong>${escapeHtml(j.human)}</strong><span class="mono">${escapeHtml(j.schedule)}</span></div><div class="command" title="${escapeAttr(j.command)}">${escapeHtml(j.command)}</div><div class="last-run">${last}</div><div class="job-actions"><button class="icon-button" data-a="run" title="Run now">▶</button><button class="icon-button" data-a="history" title="History">≡</button><button class="icon-button" data-a="edit" title="Edit">✎</button><button class="icon-button" data-a="toggle" title="${j.enabled?'Disable':'Enable'}">${j.enabled?'Ⅱ':'▷'}</button><button class="icon-button" data-a="delete" title="Delete">×</button></div>`;
  row.querySelectorAll('button').forEach(b=>b.addEventListener('click',()=>jobAction(j,b.dataset.a)));
  return row;
}

async function jobAction(job, action) {
  if(action==='edit') return openEditor(job);
  if(action==='history') return openHistory(job);
  if(action==='delete') {
    if(!confirm(`Delete “${job.name}”?\n\nA full crontab backup will be created first.`)) return;
    try { await api('/api/jobs/delete',{method:'POST',body:{id:job.id}}); toast('Job deleted'); await loadJobs(); } catch(e){toast(e.message,true)}
    return;
  }
  if(action==='toggle') {
    try { await api('/api/jobs/toggle',{method:'POST',body:{id:job.id,enabled:!job.enabled}}); toast(job.enabled?'Job disabled':'Job enabled'); await loadJobs(); } catch(e){toast(e.message,true)}
    return;
  }
  if(action==='run') {
    if(!confirm(`Run “${job.name}” now as root?`)) return;
    try { await api('/api/jobs/run',{method:'POST',body:{id:job.id}}); toast('Job started'); await loadJobs(); setTimeout(loadJobs,1800); setTimeout(loadJobs,5000); } catch(e){toast(e.message,true)}
  }
}

function openEditor(job=null) {
  $('#jobId').value=job?.id||''; $('#jobName').value=job?.name||''; $('#jobSchedule').value=job?.schedule||'17 * * * *'; $('#jobCommand').value=job?.command||''; $('#jobEnabled').value=String(job?.enabled ?? true); $('#presetSelect').value='custom';
  $('#editorTitle').textContent=job?'Edit scheduled job':'New scheduled job'; $('#importWarning').classList.toggle('hidden',!job||job.managed); $('#jobError').textContent=''; $('#editorBackdrop').classList.remove('hidden'); setTimeout(()=>$('#jobName').focus(),20);
}
function closeEditor(){ $('#editorBackdrop').classList.add('hidden'); }
$('#addBtn').addEventListener('click',()=>openEditor()); $('#closeEditorBtn').addEventListener('click',closeEditor); $('#cancelEditorBtn').addEventListener('click',closeEditor);
$('#editorBackdrop').addEventListener('click',e=>{if(e.target===$('#editorBackdrop'))closeEditor()});

$('#presetSelect').addEventListener('change',()=>{
  const map={hourly:'0 * * * *',daily:'0 2 * * *',weekly:'0 3 * * 0',monthly:'0 3 1 * *',reboot:'@reboot'}; const v=$('#presetSelect').value; if(map[v]) $('#jobSchedule').value=map[v];
});

$('#jobForm').addEventListener('submit',async e=>{
  e.preventDefault(); $('#jobError').textContent='';
  const body={id:$('#jobId').value,name:$('#jobName').value.trim(),schedule:$('#jobSchedule').value.trim(),command:$('#jobCommand').value.trim(),enabled:$('#jobEnabled').value==='true'};
  try { await api('/api/jobs/save',{method:'POST',body}); closeEditor(); toast('Job saved'); await loadJobs(); } catch(e){ $('#jobError').textContent=e.message; }
});

async function openHistory(job) {
  $('#historyTitle').textContent=job.name; $('#historyList').innerHTML='<div class="empty"><p>Loading…</p></div>'; $('#historyBackdrop').classList.remove('hidden');
  try { const data=await api('/api/history?id='+encodeURIComponent(job.id)); renderHistory(data.history||[]); } catch(e){ $('#historyList').innerHTML=`<p class="form-error">${escapeHtml(e.message)}</p>`; }
}
function renderHistory(rows){ const el=$('#historyList'); el.innerHTML=''; if(!rows.length){el.innerHTML='<div class="empty"><div class="empty-icon">≡</div><h3>No execution history</h3><p>Run this job from Cron Manager, or adopt it so scheduled executions can be tracked.</p></div>';return} for(const r of rows){const d=document.createElement('div');d.className='history-row';d.innerHTML=`<div class="history-head"><strong class="${r.exitCode===0?'run-good':'run-bad'}">${r.exitCode===0?'Success':'Exit '+r.exitCode}</strong><span>${formatDate(r.started)} · ${escapeHtml(r.source)}</span></div>${r.output?`<pre class="history-output">${escapeHtml(r.output)}</pre>`:''}`;el.appendChild(d)} }
function closeHistory(){ $('#historyBackdrop').classList.add('hidden') } $('#closeHistoryBtn').addEventListener('click',closeHistory); $('#historyBackdrop').addEventListener('click',e=>{if(e.target===$('#historyBackdrop'))closeHistory()});

async function loadBackups(){ try{const d=await api('/api/backups');const el=$('#backupList');el.innerHTML='';if(!d.backups.length){el.innerHTML='<div class="empty"><p>No backups yet. The first one appears before your first change.</p></div>';return}d.backups.forEach(name=>{const row=document.createElement('div');row.className='backup-row';row.innerHTML=`<div><code>${escapeHtml(name)}</code></div><button class="button ghost small">Restore</button>`;row.querySelector('button').onclick=()=>restoreBackup(name);el.appendChild(row)})}catch(e){toast(e.message,true)}}
async function restoreBackup(name){if(!confirm(`Restore ${name}?\n\nCron Manager will back up the current crontab before restoring.`))return;try{await api('/api/backups/restore',{method:'POST',body:{name}});toast('Crontab restored');await loadJobs();await loadBackups()}catch(e){toast(e.message,true)}}
$('#reloadBackupsBtn').addEventListener('click',loadBackups);


function downloadURL(path) {
  const a=document.createElement('a'); a.href=path; a.download=''; document.body.appendChild(a); a.click(); a.remove();
}

$('#exportJobsBtn').addEventListener('click',()=>downloadURL('/api/export'));
$('#exportCrontabBtn').addEventListener('click',()=>downloadURL('/api/export/crontab'));

$('#importFile').addEventListener('change', async ()=>{
  state.importBundle=null; $('#importBtn').disabled=true; $('#importError').textContent='';
  const file=$('#importFile').files?.[0];
  if(!file){ $('#importPreview').textContent='Choose an export file to preview it.'; return; }
  if(file.size>1024*1024){ $('#importError').textContent='Export file is too large.'; return; }
  try {
    const bundle=JSON.parse(await file.text());
    if(bundle.format!=='asustor-cron-manager' || bundle.formatVersion!==1 || !Array.isArray(bundle.jobs)) throw new Error('This is not a supported Cron Manager export.');
    state.importBundle=bundle; $('#importBtn').disabled=false;
    const when=bundle.exportedAt?` · ${formatDate(bundle.exportedAt)}`:'';
    $('#importPreview').textContent=`${bundle.jobs.length} managed job${bundle.jobs.length===1?'':'s'} · exported by v${bundle.appVersion||'?'}${when}`;
  } catch(e) { $('#importError').textContent=e.message; $('#importPreview').textContent='Could not read this export.'; }
});

$('#importBtn').addEventListener('click', async ()=>{
  if(!state.importBundle) return;
  const mode=$('#importMode').value;
  const explanation=mode==='replace-managed'?'Existing managed jobs will be removed first. Imported ADM/system lines remain untouched.':'Existing managed jobs remain; exact schedule + command duplicates are skipped.';
  if(!confirm(`Import ${state.importBundle.jobs.length} managed job(s)?\n\n${explanation}\n\nThe full current crontab will be backed up first.`)) return;
  $('#importBtn').disabled=true; $('#importError').textContent='';
  try {
    const data=await api('/api/import',{method:'POST',body:{bundle:state.importBundle,mode}});
    const r=data.result||{}; toast(`Import complete: ${r.imported||0} added, ${r.skipped||0} skipped`);
    await loadJobs();
  } catch(e) { $('#importError').textContent=e.message; }
  finally { $('#importBtn').disabled=false; }
});

async function checkUpdate(showToast=false) {
  $('#updateError').textContent=''; $('#checkUpdateBtn').disabled=true;
  try {
    const u=await api('/api/update/status'); state.update=u;
    $('#currentVersionText').textContent=`v${u.currentVersion}`;
    if(u.updateAvailable){
      $('#updateStatus').innerHTML=`Current <strong>v${escapeHtml(u.currentVersion)}</strong> · Latest <strong>v${escapeHtml(u.latestVersion)}</strong>`;
      $('#applyUpdateBtn').classList.toggle('hidden',!u.canAutoUpdate);
      $('#applyUpdateBtn').textContent=`Install v${u.latestVersion}`;
      if(!u.canAutoUpdate) $('#updateError').textContent='The release exists, but it does not contain the required verified ARM64 self-update assets.';
      if(showToast) toast(`Cron Manager v${u.latestVersion} is available`);
    } else {
      $('#updateStatus').innerHTML=`Current version: <strong>v${escapeHtml(u.currentVersion)}</strong> · Up to date`;
      $('#applyUpdateBtn').classList.add('hidden');
      if(showToast) toast('Cron Manager is up to date');
    }
  } catch(e) { $('#updateError').textContent=e.message; $('#applyUpdateBtn').classList.add('hidden'); }
  finally { $('#checkUpdateBtn').disabled=false; }
}

$('#checkUpdateBtn').addEventListener('click',()=>checkUpdate(true));
$('#applyUpdateBtn').addEventListener('click', async ()=>{
  const u=state.update; if(!u?.updateAvailable) return;
  if(!confirm(`Install Cron Manager v${u.latestVersion} from the official GitHub repository?\n\nThe ARM64 binary and its SHA-256 checksum will be downloaded and verified. Cron Manager will restart automatically.`)) return;
  $('#applyUpdateBtn').disabled=true; $('#updateError').textContent='Downloading and verifying update…';
  try {
    await api('/api/update/apply',{method:'POST',body:{confirm:true}});
    $('#updateError').textContent='Update installed. Restarting…';
    await waitForRestart(u.latestVersion);
  } catch(e) { $('#updateError').textContent=e.message; $('#applyUpdateBtn').disabled=false; }
});

async function waitForRestart(expected) {
  const deadline=Date.now()+45000;
  while(Date.now()<deadline){
    await new Promise(r=>setTimeout(r,1800));
    try {
      const st=await api('/api/status');
      if(st.version===expected){ state.version=st.version; $('#versionText').textContent=`v${st.version}`; $('#currentVersionText').textContent=`v${st.version}`; toast(`Updated to v${st.version}`); $('#applyUpdateBtn').classList.add('hidden'); $('#updateError').textContent=''; return; }
    } catch(_) {}
  }
  $('#updateError').textContent='Update was installed, but the restarted service did not answer in time. Refresh the page or check App Central.';
  $('#applyUpdateBtn').disabled=false;
}

$('#passwordForm').addEventListener('submit',async e=>{e.preventDefault();$('#passwordError').textContent='';try{await api('/api/settings/password',{method:'POST',body:{current:$('#currentPassword').value,new:$('#newPassword').value}});toast('Password changed. Please sign in again.');showAuth(false)}catch(err){$('#passwordError').textContent=err.message}});
$('#logoutBtn').addEventListener('click',async()=>{try{await api('/api/logout',{method:'POST',body:{}})}catch(_){}showAuth(false)});

$$('.nav-item').forEach(btn=>btn.addEventListener('click',()=>switchPage(btn.dataset.page,btn)));
function switchPage(page,btn){$$('.nav-item').forEach(x=>x.classList.toggle('active',x===btn));$$('.page').forEach(x=>x.classList.remove('active'));$(`#${page}Page`).classList.add('active');const meta={jobs:['Scheduled jobs','Manage the real root crontab without losing hand-written entries.'],backups:['Safety backups','Restore previous crontab states in a few clicks.'],transfer:['Export & import','Move managed jobs safely without copying ADM system tasks.'],settings:['Settings','Security, updates and local application behavior.']}[page];$('#pageTitle').textContent=meta[0];$('#pageSubtitle').textContent=meta[1];$('#addBtn').classList.toggle('hidden',page!=='jobs');$('#refreshBtn').classList.toggle('hidden',page!=='jobs');if(page==='backups')loadBackups();if(page==='settings'&&!state.update)checkUpdate(false)}
$('#refreshBtn').addEventListener('click',loadJobs); $('#searchInput').addEventListener('input',renderJobs); $('#filterSelect').addEventListener('change',renderJobs);

document.addEventListener('keydown',e=>{if(e.key==='Escape'){closeEditor();closeHistory()}});
function relativeTime(v){const t=new Date(v);const sec=Math.max(0,(Date.now()-t.getTime())/1000);if(sec<60)return'just now';if(sec<3600)return`${Math.floor(sec/60)}m ago`;if(sec<86400)return`${Math.floor(sec/3600)}h ago`;return`${Math.floor(sec/86400)}d ago`}
function formatDate(v){return new Date(v).toLocaleString()}
function escapeHtml(s=''){return String(s).replace(/[&<>'"]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[c]))}
function escapeAttr(s=''){return escapeHtml(s)}

boot();
