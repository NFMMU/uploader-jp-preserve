import{useEffect,useRef,useState}from'react';import{NavLink,Routes,Route,useNavigate,useLocation}from'react-router-dom';import PublicSite from './PublicSite';import{Archive,LayoutDashboard,Files,HardDrive,Workflow,ClipboardList,Users,ScrollText,Settings,LogOut,Search,Plus,RefreshCw,ShieldCheck,AlertTriangle,CloudUpload,ExternalLink,Menu,X,Download,CheckCircle2,Clock3,LockKeyhole}from'lucide-react';
type Obj=Record<string,any>;const apiBase='/api/v1';function App(){const[token,setToken]=useState(localStorage.getItem('archive_token')||'');const[user,setUser]=useState<Obj|null>(JSON.parse(localStorage.getItem('archive_user')||'null'));const[toast,setToast]=useState('');const[busy,setBusy]=useState(false);const[mobile,setMobile]=useState(false);const nav=useNavigate();const loc=useLocation();const[refresh,setRefresh]=useState(0);async function api(path:string,opts:RequestInit={}){const headers=new Headers(opts.headers);if(token)headers.set('Authorization','Bearer '+token);if(opts.body&&!(opts.body instanceof FormData))headers.set('Content-Type','application/json');const r=await fetch(apiBase+path,{...opts,headers});const text=await r.text();let data:Obj={};try{data=text?JSON.parse(text):{}}catch{data={message:text}}if(!r.ok)throw new Error(data.error||data.message||`HTTP ${r.status}`);return data}function notify(s:string){setToast(s);setTimeout(()=>setToast(''),3500)}async function login(e:React.FormEvent<HTMLFormElement>){e.preventDefault();setBusy(true);const f=new FormData(e.currentTarget);try{const d=await api('/auth/login',{method:'POST',body:JSON.stringify({email:f.get('email'),password:f.get('password')})});localStorage.setItem('archive_token',d.access_token);localStorage.setItem('archive_user',JSON.stringify(d.user));setToken(d.access_token);setUser(d.user);notify('ログインしました');nav('/admin')}catch(e){notify((e as Error).message)}finally{setBusy(false)}}function logout(){localStorage.removeItem('archive_token');localStorage.removeItem('archive_user');setToken('');setUser(null);nav('/admin/login')}
const links=[['/admin','概要',LayoutDashboard],['/admin/review','保全レビュー',ClipboardList],['/admin/nodes','保存ノード',HardDrive],['/admin/tasks','タスク',Workflow],['/admin/users','ユーザー管理',Users],['/admin/audit','操作履歴',ScrollText]] as const;
if(!loc.pathname.startsWith('/admin'))return <PublicSite/>;
if(!token||!user)return <div className="login-shell"><div className="login-art"><div className="brand"><div className="brand-icon"><Archive/></div><div><b>uploader.jp保全プロジェクト</b><small>アーカイブ管理</small></div></div><div className="hero"><span className="eyebrow"></span><h1>uploader.jp保全プロジェクト</h1><p>作品情報とファイルを保全します。</p><div className="hero-stats"><span><ShieldCheck/> ファイル検証</span><span><HardDrive/> 分散保存</span><span><LockKeyhole/> 権限管理</span></div></div><div className="login-foot"></div></div><div className="login-form-wrap"><form className="login-form" onSubmit={login}><span className="eyebrow"></span><h2>ログイン</h2><p>アカウント情報を入力してください。</p><label>メールアドレス<input type="email" name="email" placeholder="admin@example.jp" required autoComplete="username"/></label><label>パスワード<input type="password" name="password" placeholder="••••••••••••" required autoComplete="current-password"/></label><button className="primary wide" disabled={busy}>{busy?'認証中…':'ログインする'} <span>→</span></button><div className="login-note"><LockKeyhole size={15}/> 通信は暗号化されます</div></form></div>{toast&&<div className="toast error">{toast}</div>}</div>;
return <div className="app-shell"><aside className={'sidebar '+(mobile?'open':'')}><div className="brand"><div className="brand-icon"><Archive/></div><div><b>uploader.jp保全プロジェクト</b><small>管理画面</small></div><button className="icon-btn mobile-close" onClick={()=>setMobile(false)}><X/></button></div><div className="side-caption">メニュー</div><nav>{links.map(([to,label,Icon])=><NavLink key={to} to={to} end={to==='/admin'} onClick={()=>setMobile(false)}><Icon size={18}/><span>{label}</span>{to==='/admin/review'&&<i className="nav-dot"/>}</NavLink>)}</nav><div className="side-bottom"><div className="system-status"><span className="pulse"/><div><b>API 接続中</b><small>中央サーバー</small></div><span className="status-label">LIVE</span></div><div className="user-card"><div className="avatar">{(user.display_name||user.email||'A').slice(0,1).toUpperCase()}</div><div className="user-meta"><b>{user.display_name||user.email}</b><small>{user.role==='admin'?'管理者':user.role==='reviewer'?'確認担当者':'閲覧者'}</small></div><button className="icon-btn" title="ログアウト" onClick={logout}><LogOut size={16}/></button></div></div></aside><main className="main"><header className="topbar"><button className="icon-btn mobile-menu" onClick={()=>setMobile(true)}><Menu/></button><div className="breadcrumbs">管理 <span>/</span> <b>{links.find(l=>l[0]===loc.pathname)?.[1]||'詳細'}</b></div><div className="top-actions"><span className="secure-tag"><span className="pulse"/> </span><button className="icon-btn" title="再読み込み" onClick={()=>setRefresh(x=>x+1)}><RefreshCw size={17}/></button><div className="top-avatar">{(user.display_name||user.email||'A').slice(0,1).toUpperCase()}</div></div></header><div className="page"><Routes><Route path="/admin" element={<Dashboard api={api} refresh={refresh} notify={notify} token={token}/>}/><Route path="/admin/review" element={<ReviewWorkspace api={api} refresh={refresh} notify={notify} role={user.role} token={token} bumpRefresh={()=>setRefresh(x=>x+1)} initialTab="files"/>}/><Route path="/admin/artifacts" element={<ReviewWorkspace api={api} refresh={refresh} notify={notify} role={user.role} token={token} bumpRefresh={()=>setRefresh(x=>x+1)} initialTab="files"/>}/><Route path="/admin/nodes" element={<Nodes api={api} refresh={refresh} notify={notify}/>}/><Route path="/admin/tasks" element={<Tasks api={api} refresh={refresh} notify={notify} role={user.role}/>}/><Route path="/admin/requests" element={<ReviewWorkspace api={api} refresh={refresh} notify={notify} role={user.role} token={token} bumpRefresh={()=>setRefresh(x=>x+1)} initialTab="requests"/>}/><Route path="/admin/users" element={<UsersPage api={api} refresh={refresh} notify={notify} role={user.role}/>}/><Route path="/admin/audit" element={<Audit api={api} refresh={refresh}/>}/><Route path="*" element={<Dashboard api={api} refresh={refresh} notify={notify} token={token}/>}/></Routes></div><footer><b>uploader.jp保全プロジェクト</b> <span>·</span> 運営：全国もちもちうどん連盟 <span>·</span> © 2026 全国もちもちうどん連盟 <span className="footer-right"><a href="mailto:uploaderjp-archive@udonya.moe">uploaderjp-archive@udonya.moe</a></span></footer></main>{toast&&<div className="toast">{toast}</div>}</div>}
function useData(api:(p:string,o?:RequestInit)=>Promise<Obj>,path:string,refresh:number){const[data,setData]=useState<Obj>({items:[]});const[error,setError]=useState('');const[loading,setLoading]=useState(true);useEffect(()=>{let live=true;setLoading(true);api(path).then(d=>{if(live){setData(d);setError('')}}).catch(e=>{if(live)setError(e.message)}).finally(()=>live&&setLoading(false));return()=>{live=false}},[path,refresh]);return{data,error,loading}}
function Heading({eyebrow,title,desc,actions}:{eyebrow:string,title:string,desc:string,actions?:React.ReactNode}){return <div className="page-heading"><div><div className="eyebrow">{eyebrow}</div><h1>{title}</h1><p>{desc}</p></div>{actions&&<div className="heading-actions">{actions}</div>}</div>}
function Stat({label,value,sub,icon:Icon,tone='blue'}:{label:string,value:React.ReactNode,sub:string,icon:any,tone?:string}){return <div className="stat-card"><div className={'stat-icon '+tone}><Icon size={19}/></div><div className="stat-label">{label}</div><div className="stat-value">{value}</div><div className="stat-sub">{sub}</div></div>}
function Dashboard({api,refresh,notify,token}:any){const a=useData(api,'/artifacts',refresh),f=useData(api,'/files',refresh),n=useData(api,'/nodes',refresh),t=useData(api,'/tasks',refresh),r=useData(api,'/public-requests?status=pending',refresh);const files=f.data.items||[],nodes=n.data.items||[],tasks=t.data.items||[];const degraded=files.filter((x:Obj)=>['degraded','missing','error'].includes(x.preservation_status)).length;const failed=tasks.filter((x:Obj)=>x.status==='failed').length;return <><Heading eyebrow="" title="保全状況" desc="保存状況と要対応項目" actions={<><button className="secondary" onClick={()=>downloadExport('json',token,notify)}><Download size={16}/> 一覧を出力</button><button className="primary" onClick={()=>api('/admin/placements/reconcile',{method:'POST'}).then(()=>notify('配置の再調整を実行しました')).catch((e:Error)=>notify(e.message))}><RefreshCw size={16}/> 保存先を調整</button></>}/><div className="notice"><div className="notice-icon"><ShieldCheck size={19}/></div><div><b>保存と公開は別です</b><p>権利未確認のファイルは、保存しても公開しません。</p></div><span className="notice-tag"></span></div><div className="stat-grid"><Stat label="作品数" value={a.data.total??(a.data.items||[]).length} sub="登録済み" icon={Files}/><Stat label="保存ファイル" value={files.length} sub="登録ファイル数" icon={Archive} tone="violet"/><Stat label="稼働ノード" value={`${nodes.filter((x:Obj)=>x.status==='online').length} / ${nodes.length}`} sub="稼働状況" icon={HardDrive} tone="green"/><Stat label="要対応" value={failed} sub="失敗したタスク" icon={Workflow} tone={failed?'red':'amber'}/></div><div className="content-grid"><section className="panel"><div className="panel-head"><div><h2>保存状態</h2><p>ファイルの状態</p></div><span className="live-chip"><span className="pulse"/> </span></div><div className="health-list">{[['verified','検証済み','green'],['queued','保存待ち','blue'],['degraded','劣化・保存先不足','amber'],['missing','実体不明','red'],['error','エラー','red']].map(([key,label,tone])=>{const count=files.filter((x:Obj)=>x.preservation_status===key).length;return <div className="health-row" key={key}><span className={'health-dot '+tone}/><span>{label}</span><b>{count}</b><div className="mini-bar"><i className={tone} style={{width:`${files.length?count/files.length*100:0}%`}}/></div></div>})}</div><div className="panel-foot"><span>要対応: <b>{degraded}</b></span><NavLink to="/admin/review?tab=files">一覧を見る →</NavLink></div></section><section className="panel"><div className="panel-head"><div><h2>操作</h2><p>よく使う機能</p></div></div><div className="quick-actions"><NavLink to="/admin/review?tab=files" className="quick-card"><div className="quick-icon blue"><Plus/></div><div><b>作品を登録</b><small>情報を登録して保存</small></div><span>→</span></NavLink><NavLink to="/admin/nodes" className="quick-card"><div className="quick-icon green"><HardDrive/></div><div><b>ノードを確認</b><small>容量と稼働状況</small></div><span>→</span></NavLink><NavLink to="/admin/review?tab=requests" className="quick-card"><div className="quick-icon amber"><ClipboardList/></div><div><b>申請を確認</b><small>未処理 {r.data.total??(r.data.items||[]).length} 件</small></div><span>→</span></NavLink><NavLink to="/admin/tasks" className="quick-card"><div className="quick-icon violet"><Workflow/></div><div><b>タスク</b><small>保存・検証</small></div><span>→</span></NavLink></div></section></div><section className="panel table-panel"><div className="panel-head"><div><h2>最近のタスク</h2><p>処理状況</p></div><NavLink to="/admin/tasks" className="text-link">一覧 →</NavLink></div><TaskTable items={tasks.slice(0,6)} loading={t.loading} error={t.error}/></section></>}
function Status({value}:{value:string}){const map:Record<string,[string,string]>={verified:['検証済み','green'],stored:['保存済み','green'],queued:['待機中','blue'],running:['実行中','blue'],succeeded:['成功','green'],completed:['完了','green'],failed:['失敗','red'],degraded:['劣化','amber'],missing:['実体不明','red'],deleted:['削除済み','gray'],error:['エラー','red'],online:['オンライン','green'],offline:['オフライン','red'],pending:['未確認','amber'],in_review:['確認中','blue'],allowed:['再配布可','green'],restricted:['制限あり','red'],unknown:['不明','amber'],in_progress:['確認中','blue'],uncollected:['未取得','gray'],accepted:['承認','green'],rejected:['却下','red'],closed:['終了','gray']};const [label,tone]=map[value]||[value||'—','gray'];return <span className={'badge '+tone}><i/>{label}</span>}
function Table({headers,children,empty='データがありません'}:{headers:string[],children:React.ReactNode,empty?:string}){return <div className="table-wrap"><table><thead><tr>{headers.map(h=><th key={h}>{h}</th>)}</tr></thead><tbody>{children||<tr><td colSpan={headers.length} className="empty">{empty}</td></tr>}</tbody></table></div>}
function TaskTable({items,loading,error}:{items:Obj[],loading:boolean,error:string}){return <Table headers={['タスクID','種別','状態','優先度','更新日時']}>{loading?<tr><td colSpan={5} className="empty">読み込み中…</td></tr>:error?<tr><td colSpan={5} className="empty">{error}</td></tr>:items.length?items.map((x,i)=><tr key={x.id||i}><td><code>{String(x.id||'').slice(0,8)||'—'}</code></td><td>{x.task_type||'—'}</td><td><Status value={x.status}/></td><td><span className="priority">{x.priority??'—'}</span></td><td>{date(x.updated_at||x.created_at)}</td></tr>):<tr><td colSpan={5} className="empty">タスクがありません</td></tr>}</Table>}
async function downloadExport(format:string,token:string,notify:(s:string)=>void){try{const r=await fetch(`/api/v1/export?format=${format}`,{headers:{Authorization:`Bearer ${token}`}});if(!r.ok)throw new Error('エクスポートに失敗しました');const blob=await r.blob();const url=URL.createObjectURL(blob);const a=document.createElement('a');a.href=url;a.download=`archive-catalog.${format}`;a.click();URL.revokeObjectURL(url);notify('カタログを出力しました')}catch(e){notify((e as Error).message)}}function date(v:string){return v?new Date(v).toLocaleString('ja-JP',{dateStyle:'medium',timeStyle:'short'}):'—'}function bytes(v:number){if(v==null)return'—';const u=['B','KB','MB','GB','TB','PB'];let n=v,i=0;while(n>=1024&&i<u.length-1){n/=1024;i++}return`${n.toFixed(i?1:0)} ${u[i]}`}
function ReviewWorkspace({api,refresh,notify,role,token,bumpRefresh,initialTab='files'}:any){const loc=useLocation();const nav=useNavigate();const queryTab=new URLSearchParams(loc.search).get('tab');const tab=(queryTab==='requests'||(!queryTab&&initialTab==='requests'))?'requests':'files';function choose(next:'files'|'requests'){nav(`/admin/review?tab=${next}`)}return <><div className="review-workspace-tabs" role="tablist" aria-label="保全レビュー"><button type="button" role="tab" aria-selected={tab==='files'} className={tab==='files'?'active':''} onClick={()=>choose('files')}><Files size={16}/> 作品・ファイル</button><button type="button" role="tab" aria-selected={tab==='requests'} className={tab==='requests'?'active':''} onClick={()=>choose('requests')}><ClipboardList size={16}/> 申請・保全レビュー</button></div>{tab==='files'?<Catalog api={api} refresh={refresh} notify={notify} role={role} token={token}/>:<Requests api={api} refresh={refresh} notify={notify} role={role} bumpRefresh={bumpRefresh}/>}</>}
function Catalog({api,refresh,notify,role,token}:any){
  const [unused,setRefreshDummy]=useState(0);
  const loc=useLocation();
  const nav=useNavigate();
  const params=new URLSearchParams(loc.search);
  const linkedFileID=params.get('file_id')||'';
  const linkedArtifactID=params.get('artifact_id')||'';
  const [query,setQuery]=useState('');
  const [preservationFilter,setPreservationFilter]=useState('');
  const [policyFilter,setPolicyFilter]=useState('');
  const [publicationFilter,setPublicationFilter]=useState('');
  const [selectedIDs,setSelectedIDs]=useState<string[]>([]);
  const [bulkPolicy,setBulkPolicy]=useState('restricted');
  const [relatedRequestID,setRelatedRequestID]=useState('');
  const [modal,setModal]=useState(false);
  const [detail,setDetail]=useState<Obj|null>(null);
  const [metadata,setMetadata]=useState<Obj|null>(null);
  const [metaForm,setMetaForm]=useState({artifact_title:'',creator_name:'',artifact_source_url:'',artifact_description:'',original_filename:'',media_type:'',source_url:''});
  const [artifact,setArtifact]=useState<Obj|null>(null);
  const [pendingFileId,setPendingFileId]=useState('');
  const [busy,setBusy]=useState(false);
  const [upload,setUpload]=useState<File|null>(null);
  const fileInputRef=useRef<HTMLInputElement|null>(null);
  const [sourceURL,setSourceURL]=useState('');
  const [acceptSourceTerms,setAcceptSourceTerms]=useState(false);
  const [listingURL,setListingURL]=useState('');
  const [acceptListingTerms,setAcceptListingTerms]=useState(false);
  const [discovering,setDiscovering]=useState(false);
  const [candidates,setCandidates]=useState<Obj[]>([]);
  const [selectedCandidates,setSelectedCandidates]=useState<string[]>([]);
  const [batchBusy,setBatchBusy]=useState(false);
  const [batchStatus,setBatchStatus]=useState('');
  const [priority,setPriority]=useState('3');
  const [policy,setPolicy]=useState('pending');
  const [pub,setPub]=useState(false);
  const [form,setForm]=useState({title:'',creator_name:'',source_url:'',description:''});
  let filePath='/files?limit=100';
  if(query.trim())filePath+='&q='+encodeURIComponent(query.trim());
  if(preservationFilter)filePath+='&preservation_status='+encodeURIComponent(preservationFilter);
  if(policyFilter)filePath+='&redistribution_policy='+encodeURIComponent(policyFilter);
  if(publicationFilter)filePath+='&publication='+encodeURIComponent(publicationFilter);
  if(linkedArtifactID)filePath+='&artifact_id='+encodeURIComponent(linkedArtifactID);
  const f=useData(api,filePath,refresh+unused);
  const requestData=useData(api,'/public-requests?limit=100',refresh+unused);
  const files:Obj[]=f.data.items||[];
  const requests:Obj[]=(requestData.data.items||[]).filter((x:Obj)=>x.status==='accepted'||x.status==='in_review'||x.status==='pending');

  useEffect(()=>{
    if(!linkedFileID||f.loading)return;
    const target=(f.data.items||[]).find((x:Obj)=>x.id===linkedFileID);
    if(target){setDetail(target);setPolicy(target.redistribution_policy||'unknown');setPub(!!target.public_download_enabled);nav('/admin/review?tab=files',{replace:true});}
  },[linkedFileID,f.loading,f.data.items]);

  function openMetadata(x:Obj){
    setMetadata(x);
    setMetaForm({
      artifact_title:x.artifact_title||'',creator_name:x.creator_name||'',
      artifact_source_url:x.artifact_source_url||'',artifact_description:x.artifact_description||'',
      original_filename:x.original_filename||'',media_type:x.media_type||'',source_url:x.source_url||''
    });
  }
  async function saveMetadata(e:React.FormEvent){
    e.preventDefault();if(!metadata)return;
    const reason=window.prompt('メタデータの変更理由を入力してください');
    if(!reason||reason.trim().length<3)return;
    setBusy(true);
    try{
      await api(`/artifacts/${metadata.artifact_id}`,{method:'PATCH',body:JSON.stringify({
        title:metaForm.artifact_title,creator_name:metaForm.creator_name,
        source_url:metaForm.artifact_source_url,description:metaForm.artifact_description,reason:reason.trim()
      })});
      await api(`/files/${metadata.id}/metadata`,{method:'PATCH',body:JSON.stringify({
        original_filename:metaForm.original_filename,media_type:metaForm.media_type,
        source_url:metaForm.source_url,reason:reason.trim()
      })});
      notify('作品・ファイルのメタデータを更新しました');setMetadata(null);setRefreshDummy(x=>x+1);
    }catch(e){notify((e as Error).message)}finally{setBusy(false)}
  }
  async function runBulk(action:'policy'|'publication'|'delete',extra:Obj={}){
    if(selectedIDs.length===0){notify('先にファイルを選択してください');return;}
    if(action==='delete'&&!relatedRequestID){notify('削除を承認した申請を選択してください');return;}
    if(action==='delete'){const linked=requests.find((r:Obj)=>r.id===relatedRequestID);if(!linked||linked.request_type!=='deletion'||linked.status!=='accepted'){notify('承認済みの削除申請を選択してください');return;}}
    const reason=window.prompt(action==='delete'?'削除処理の理由を入力してください':'一括変更の理由を入力してください');
    if(!reason||reason.trim().length<3)return;
    try{
      const body:Obj={file_ids:selectedIDs,action,reason:reason.trim(),...extra};
      if(relatedRequestID)body.request_id=relatedRequestID;
      const result=await api('/files/bulk-action',{method:'POST',body:JSON.stringify(body)});
      notify(`${result.count} 件を一括処理しました`);setSelectedIDs([]);setRefreshDummy(x=>x+1);
    }catch(e){notify((e as Error).message)}
  }
  async function createDeletionRequests(){
    if(selectedIDs.length===0)return;
    const reason=window.prompt('削除申請の理由を入力してください（5文字以上）');
    if(!reason||reason.trim().length<5)return;
    if(!window.confirm(`選択した ${selectedIDs.length} 件について削除申請を作成します。実際の削除は申請を承認した後に始まります。続行しますか？`))return;
    try{
      const result=await api('/admin/deletion-requests',{method:'POST',body:JSON.stringify({file_ids:selectedIDs,reason:reason.trim()})});
      notify(`${result.count} 件の削除申請を作成しました。申請・保全レビューで承認してください。`);
      setSelectedIDs([]);nav('/admin/review?tab=requests');
    }catch(e){notify((e as Error).message)}
  }
  function toggleSelected(id:string,checked:boolean){setSelectedIDs(old=>checked?(old.includes(id)?old:[...old,id]):old.filter(x=>x!==id));}
  function toggleAll(checked:boolean){const selectable=files.filter((x:Obj)=>!x.deletion_requested_at&&!x.deleted_at).map((x:Obj)=>x.id);setSelectedIDs(old=>checked?Array.from(new Set([...old,...selectable])):old.filter(id=>!selectable.includes(id)));}

  async function create(e:React.FormEvent){e.preventDefault();setBusy(true);try{const a=await api('/artifacts',{method:'POST',body:JSON.stringify(form)});notify('作品を登録しました');setModal(false);setForm({title:'',creator_name:'',source_url:'',description:''});setArtifact(a);setPendingFileId('');setRefreshDummy(x=>x+1)}catch(e){notify((e as Error).message)}finally{setBusy(false)}}
  async function addFile(e:React.FormEvent){e.preventDefault();const url=sourceURL.trim();if(!artifact||(!upload&&!url)||(upload&&url)||(url&&!acceptSourceTerms)||policy==='pending')return;setBusy(true);try{let fileId=pendingFileId;if(!fileId){const file=await api(`/artifacts/${artifact.id}/files`,{method:'POST',body:JSON.stringify({original_filename:upload?.name||'取得待ち',size_bytes:upload?.size||0,media_type:upload?.type||'application/octet-stream',source_url:url||undefined,redistribution_policy:policy,public_download_enabled:false})});fileId=file.id;setPendingFileId(fileId)}let ingestResult:any;if(upload){const fd=new FormData();fd.append('file',upload);fd.append('priority',priority);ingestResult=await api(`/files/${fileId}/ingest`,{method:'POST',body:fd})}else{ingestResult=await api(`/files/${fileId}/ingest-source`,{method:'POST',body:JSON.stringify({source_url:url,accept_terms:acceptSourceTerms,priority:Number(priority)})})}const createdTasks=Array.isArray(ingestResult?.tasks)?ingestResult.tasks.length:0;notify(ingestResult?.source_fetch_task?`取得タスクを ${ingestResult.source_fetch_task.node_name||'agent'} に登録しました。取得成功後に保存タスクが作成されます。`:createdTasks?`登録しました。保存タスク ${createdTasks} 件を作成しました。`:'登録しました。保全処理を受け付けました。');setArtifact(null);setPendingFileId('');setUpload(null);setSourceURL('');setAcceptSourceTerms(false);setListingURL('');setAcceptListingTerms(false);setCandidates([]);setSelectedCandidates([]);setBatchStatus('');setRefreshDummy(x=>x+1)}catch(e){notify((e as Error).message)}finally{setBusy(false)}}
  async function discoverListing(){if(!artifact||!listingURL.trim()||!acceptListingTerms)return;setDiscovering(true);setCandidates([]);setSelectedCandidates([]);setBatchStatus('一覧を確認しています…');try{const result=await api(`/artifacts/${artifact.id}/discover-source-files`,{method:'POST',body:JSON.stringify({listing_url:listingURL.trim(),accept_terms:acceptListingTerms})});const found=(result.items||[]) as Obj[];setCandidates(found);setSelectedCandidates(found.map((x:Obj)=>x.url));setBatchStatus(`${result.pages_scanned} ページから ${found.length} 件を検出しました。`+(result.truncated?'上限100件に達したため、残りは次回確認してください。':''))}catch(e){setBatchStatus((e as Error).message)}finally{setDiscovering(false)}}
  async function importSelected(){if(!artifact||selectedCandidates.length===0||policy==='pending')return;const chosen=candidates.filter((x:Obj)=>selectedCandidates.includes(x.url));setBatchBusy(true);let ok=0;let fetchTaskCount=0;let taskCount=0;const failures:string[]=[];for(let i=0;i<chosen.length;i++){const item=chosen[i];setBatchStatus(`取得タスク登録中 ${i+1}/${chosen.length}: ${item.label||item.url}`);try{const file=await api(`/artifacts/${artifact.id}/files`,{method:'POST',body:JSON.stringify({original_filename:(item.label||'取得待ち').slice(0,500),size_bytes:0,media_type:'application/octet-stream',source_url:item.url,redistribution_policy:policy})});const queued=await api(`/files/${file.id}/ingest-source`,{method:'POST',body:JSON.stringify({source_url:item.url,accept_terms:true,priority:Number(priority)})});ok++;if(queued.source_fetch_task)fetchTaskCount++;taskCount+=Array.isArray(queued?.tasks)?queued.tasks.length:0}catch(e){failures.push(`${item.label||item.url}: ${(e as Error).message}`)}}setBatchBusy(false);setSelectedCandidates([]);setCandidates([]);setBatchStatus(`完了: ${ok} 件受付、agent取得タスク ${fetchTaskCount} 件登録、保存タスク ${taskCount} 件作成、${failures.length} 件失敗。`+(fetchTaskCount?'保存タスクは各 agent の取得成功後に作成されます。 ':'')+(failures.length?`最初の失敗: ${failures[0]}`:''));notify(`一括保全: ${ok} 件受付、agent取得タスク ${fetchTaskCount} 件登録、失敗 ${failures.length} 件`);setRefreshDummy(x=>x+1)}
  async function savePolicy(){if(!detail)return;const reason=window.prompt('変更理由を入力してください');if(!reason||reason.trim().length<3)return;try{await api(`/files/${detail.id}/policy`,{method:'PATCH',body:JSON.stringify({redistribution_policy:policy,reason:reason.trim()})});await api(`/files/${detail.id}/publication`,{method:'PATCH',body:JSON.stringify({enabled:pub,reason:reason.trim()})});notify('公開ポリシーを更新しました');setDetail(null);setRefreshDummy(x=>x+1)}catch(e){notify((e as Error).message)}}

  return <>
    <Heading eyebrow="" title="作品・ファイル" desc="メタデータ、保全状態、公開条件を検索・一括管理" actions={<><button className="secondary" onClick={()=>downloadExport('csv',token,notify)}><Download size={16}/> CSV出力</button>{role==='admin'&&<button className="primary" onClick={()=>setModal(true)}><Plus size={16}/> 作品を登録</button>}</>}/>
    <div className="filter-bar file-filter-bar">
      <div className="search-box"><Search size={17}/><input value={query} onChange={e=>setQuery(e.target.value)} placeholder="作品名・ファイル名・作者・IDで検索"/></div>
      <select aria-label="保全状態で絞り込み" value={preservationFilter} onChange={e=>setPreservationFilter(e.target.value)}><option value="">すべての保全状態</option><option value="uncollected">未取得</option><option value="queued">待機中</option><option value="stored">保存済み</option><option value="verified">検証済み</option><option value="degraded">劣化</option><option value="missing">実体不明</option><option value="error">エラー</option></select>
      <select aria-label="公開条件で絞り込み" value={policyFilter} onChange={e=>setPolicyFilter(e.target.value)}><option value="">すべての公開条件</option><option value="allowed">再配布可</option><option value="restricted">制限付き</option><option value="in_progress">確認中</option><option value="pending">未確認</option><option value="unknown">不明</option></select>
      <select aria-label="公開状態で絞り込み" value={publicationFilter} onChange={e=>setPublicationFilter(e.target.value)}><option value="">公開状態：すべて</option><option value="public">公開中</option><option value="private">非公開</option><option value="restricted">制限付き</option><option value="pending">未確認</option><option value="unknown">条件不明</option></select>
      <span className="result-count">{f.data.total??files.length} 件</span>
    </div>
    {selectedIDs.length>0&&role!=='viewer'&&<section className="bulk-toolbar panel">
      <div className="bulk-toolbar-title"><b>{selectedIDs.length} 件を選択中</b><button className="small-btn" onClick={()=>setSelectedIDs([])}>選択解除</button></div>
      <div className="bulk-toolbar-controls">
        <label>関連申請<select value={relatedRequestID} onChange={e=>setRelatedRequestID(e.target.value)}><option value="">申請に紐づけない</option>{requests.map((r:Obj)=><option key={r.id} value={r.id}>{r.request_type==='deletion'?'削除':r.request_type==='takedown'?'公開停止':r.request_type==='preservation'?'保全':'移設先'} / {r.linked_file_name||r.linked_artifact_title||r.title||String(r.id).slice(0,8)} / {r.status==='accepted'?'承認':r.status==='in_review'?'確認中':'未処理'}</option>)}</select></label>
        <label>公開条件<select value={bulkPolicy} onChange={e=>setBulkPolicy(e.target.value)}><option value="allowed">再配布可</option><option value="restricted">制限付き</option><option value="in_progress">確認中</option><option value="pending">未確認</option><option value="unknown">不明</option></select></label>
        <button className="secondary" onClick={()=>runBulk('policy',{redistribution_policy:bulkPolicy})}>公開条件を一括変更</button>
        {role==='admin'&&<><button className="primary" onClick={()=>runBulk('publication',{enabled:true})}>一括公開</button><button className="secondary" onClick={()=>runBulk('publication',{enabled:false})}>一括公開停止</button><button className="danger-btn" onClick={createDeletionRequests}>削除申請を作成</button><button className="danger-btn" onClick={()=>runBulk('delete')}>承認済み申請で削除</button></>}
      </div>
      <p className="muted">「削除申請を作成」は申請を未処理で登録します。申請・保全レビューで管理者が承認するとノード削除タスクが開始します。承認済みの申請を使った削除も可能です。</p>
    </section>}
    <section className="panel table-panel"><Table headers={['','作品 / ファイル','保全状態','公開条件 / 状態','サイズ','登録日時','操作']}>
      {f.loading?<tr><td colSpan={7} className="empty">読み込み中…</td></tr>:f.error?<tr><td colSpan={7} className="empty">{f.error}</td></tr>:files.length? <>
        <tr className="select-all-row"><td><input type="checkbox" aria-label="表示中のファイルをすべて選択" checked={files.filter((x:Obj)=>!x.deletion_requested_at&&!x.deleted_at).length>0&&files.filter((x:Obj)=>!x.deletion_requested_at&&!x.deleted_at).every((x:Obj)=>selectedIDs.includes(x.id))} disabled={role==='viewer'} onChange={e=>toggleAll(e.target.checked)}/></td><td colSpan={6} className="muted">表示中の {files.length} 件を選択</td></tr>
        {files.map((x:Obj)=><tr key={x.id} className={selectedIDs.includes(x.id)?'selected-row':''}><td><input type="checkbox" aria-label={`${x.original_filename}を選択`} checked={selectedIDs.includes(x.id)} disabled={role==='viewer'||!!x.deletion_requested_at||!!x.deleted_at} onChange={e=>toggleSelected(x.id,e.target.checked)}/></td><td><div className="file-cell"><div className="file-icon"><Files size={17}/></div><div><b>{x.artifact_title||'作品名未設定'}</b><small>作者：{x.creator_name||'不明'}</small><span>{x.original_filename||'名称未設定'}</span><small><code>{x.id}</code></small></div></div></td><td><Status value={x.deleted_at?'deleted':x.preservation_status}/>{x.deletion_requested_at&&!x.deleted_at&&<small className="cell-sub">削除処理中</small>}</td><td><Status value={x.redistribution_policy}/><small className="cell-sub">{x.public_download_enabled?'公開フラグON':'非公開'}</small></td><td>{bytes(x.size_bytes)}</td><td>{date(x.created_at)}</td><td><div className="row-actions"><button className="small-btn" disabled={role==='viewer'} onClick={()=>{setDetail(x);setPolicy(x.redistribution_policy||'unknown');setPub(!!x.public_download_enabled)}}>公開設定</button><button className="small-btn" disabled={role==='viewer'||!!x.deletion_requested_at||!!x.deleted_at} onClick={()=>openMetadata(x)}>メタデータ</button></div></td></tr>)}
      </>:<tr><td colSpan={7} className="empty">ファイルがありません</td></tr>}
    </Table></section>
    {modal&&<Modal title="作品を登録" close={()=>setModal(false)}><form onSubmit={create} className="form-stack"><label>作品名 *<input required value={form.title} onChange={e=>setForm({...form,title:e.target.value})}/></label><label>作者・配布者<input value={form.creator_name} onChange={e=>setForm({...form,creator_name:e.target.value})}/></label><label>元ページURL *<input required type="url" placeholder="https://www.uploader.jp/..." value={form.source_url} onChange={e=>setForm({...form,source_url:e.target.value})}/></label><label>説明<textarea rows={3} value={form.description} onChange={e=>setForm({...form,description:e.target.value})}/></label><div className="modal-actions"><button type="button" className="secondary" onClick={()=>setModal(false)}>キャンセル</button><button className="primary" disabled={busy}>{busy?'登録中…':'次へ'}</button></div></form></Modal>}
    {artifact&&<Modal title="ファイルを追加・保全" close={()=>{setArtifact(null);setPendingFileId('');setListingURL('');setAcceptListingTerms(false);setCandidates([]);setSelectedCandidates([]);setBatchStatus('')}}><form className="form-stack" onSubmit={addFile}><p className="muted">作品: <b>{artifact.title}</b></p><label>手元のファイル<input ref={fileInputRef} type="file" onChange={e=>{setUpload(e.target.files?.[0]||null);if(e.target.files?.[0]){setSourceURL('');setAcceptSourceTerms(false);setListingURL('');setAcceptListingTerms(false);setCandidates([]);setSelectedCandidates([]);setPendingFileId('')}}}/><small>最大5 GiB。保存先は2ノード必要です。</small></label><label>または uploader.jp のダウンロードURL<input type="url" value={sourceURL} onChange={e=>{setSourceURL(e.target.value);if(e.target.value.trim()){setUpload(null);setListingURL('');setAcceptListingTerms(false);setCandidates([]);setSelectedCandidates([]);if(fileInputRef.current)fileInputRef.current.value=''}else setPendingFileId('')}} placeholder="https://uu.getuploader.com/名前/download/123"/><small>個別ファイル用。パスワードや画像認証の回避は行いません。</small></label>{sourceURL.trim()&&<label className="check-label"><input type="checkbox" checked={acceptSourceTerms} onChange={e=>setAcceptSourceTerms(e.target.checked)}/> uploader.jp の利用規約を確認し、同意しました</label>}<div className="form-divider"/><label>またはアップローダーの一覧URL<input type="url" value={listingURL} onChange={e=>{setListingURL(e.target.value);if(e.target.value.trim()){setSourceURL('');setAcceptSourceTerms(false);setUpload(null);setPendingFileId('');if(fileInputRef.current)fileInputRef.current.value=''}setCandidates([]);setSelectedCandidates([]);setBatchStatus('')}} placeholder="https://uu.getuploader.com/名前/"/><small>一覧ページを調べ、掲載ファイルを選んで順番に取得します（最大100件・最大10ページ）。</small></label>{listingURL.trim()&&<label className="check-label"><input type="checkbox" checked={acceptListingTerms} onChange={e=>setAcceptListingTerms(e.target.checked)}/> uploader.jp の利用規約を確認しました</label>}<div className="modal-actions"><button type="button" className="secondary" disabled={!listingURL.trim()||!acceptListingTerms||discovering||batchBusy} onClick={discoverListing}>{discovering?'確認中…':'一覧を調べる'}</button></div>{batchStatus&&<p className="muted">{batchStatus}</p>}{candidates.length>0&&<div className="candidate-list"><div className="candidate-list-head"><b>検出したファイル ({candidates.length})</b><button type="button" className="small-btn" onClick={()=>setSelectedCandidates(selectedCandidates.length===candidates.length?[]:candidates.map((x:Obj)=>x.url))}>{selectedCandidates.length===candidates.length?'選択解除':'全件選択'}</button></div>{candidates.map((x:Obj)=><label className="candidate-row" key={x.url}><input type="checkbox" checked={selectedCandidates.includes(x.url)} onChange={e=>setSelectedCandidates(e.target.checked?[...selectedCandidates,x.url]:selectedCandidates.filter(u=>u!==x.url))}/><span>{x.label||x.url}<small>{x.url}</small></span></label>)}<button type="button" className="primary wide" disabled={batchBusy||selectedCandidates.length===0||policy==='pending'} onClick={importSelected}>{batchBusy?'取得タスク登録中…':`選択した ${selectedCandidates.length} 件の取得タスクを登録`}</button></div>}<label>公開条件<select value={policy} disabled={busy||batchBusy} onChange={e=>setPolicy(e.target.value)}><option value="pending">未確認（保全不可）</option><option value="unknown">不明</option><option value="in_progress">確認中</option><option value="allowed">許可済み</option><option value="restricted">制限あり</option></select></label><p className="muted">保全と一般公開は別の判定です。公開条件が未確認のファイルは保全を開始できません。</p><label>保存優先度（0〜5）<select value={priority} onChange={e=>setPriority(e.target.value)}>{[5,4,3,2,1,0].map(x=><option key={x} value={x}>{x}</option>)}</select></label><div className="modal-actions"><button type="button" className="secondary" onClick={()=>{setArtifact(null);setPendingFileId('');setListingURL('');setAcceptListingTerms(false);setCandidates([]);setSelectedCandidates([]);setBatchStatus('')}}>キャンセル</button><button className="primary" disabled={busy||(!upload&&!sourceURL.trim())||(!!upload&&!!sourceURL.trim())||(!!sourceURL.trim()&&!acceptSourceTerms)||policy==='pending'}><CloudUpload size={16}/>{busy?'処理中…':sourceURL.trim()?'取得して保全開始':'登録して保全開始'}</button></div></form></Modal>}
    {detail&&<Modal title="公開設定" close={()=>setDetail(null)}><div className="detail-summary"><b>{detail.artifact_title||detail.original_filename}</b><small>{detail.original_filename}</small><small>{detail.id}</small>{detail.source_url&&<small><a href={detail.source_url} target="_blank" rel="noreferrer">取得元URLを開く</a></small>}{detail.deleted_at&&<div className="policy-warning"><LockKeyhole size={16}/> 削除済みのファイルです。</div>}{detail.deletion_requested_at&&!detail.deleted_at&&<div className="policy-warning"><LockKeyhole size={16}/> ノードへの削除タスクを処理中です。</div>}<div className="policy-warning"><LockKeyhole size={16}/> 公開は権利確認後に有効にしてください。</div></div><div className="form-stack"><label>公開条件<select value={policy} onChange={e=>setPolicy(e.target.value)}><option value="unknown">不明</option><option value="pending">未確認（保全不可）</option><option value="in_progress">確認中</option><option value="allowed">許可済み</option><option value="restricted">制限あり</option></select></label><label className="check-label"><input type="checkbox" checked={pub} onChange={e=>setPub(e.target.checked)}/> 一般公開する</label><p className="muted">公開には再配布可の確認、検証済み状態、オンライン保存先が必要です。</p><div className="modal-actions"><button className="secondary" onClick={()=>setDetail(null)}>閉じる</button><button className="primary" onClick={savePolicy} disabled={!!detail.deleted_at||!!detail.deletion_requested_at}>設定を保存</button></div></div></Modal>}
    {metadata&&<Modal title="作品・ファイルのメタデータ" close={()=>setMetadata(null)}><form className="form-stack" onSubmit={saveMetadata}><label>作品名 *<input required value={metaForm.artifact_title} onChange={e=>setMetaForm({...metaForm,artifact_title:e.target.value})}/></label><label>作者・配布者<input value={metaForm.creator_name} onChange={e=>setMetaForm({...metaForm,creator_name:e.target.value})}/></label><label>作品の元ページURL *<input required type="url" value={metaForm.artifact_source_url} onChange={e=>setMetaForm({...metaForm,artifact_source_url:e.target.value})}/></label><label>作品の説明<textarea rows={3} value={metaForm.artifact_description} onChange={e=>setMetaForm({...metaForm,artifact_description:e.target.value})}/></label><div className="form-divider"/><label>ファイル名 *<input required value={metaForm.original_filename} onChange={e=>setMetaForm({...metaForm,original_filename:e.target.value})}/></label><label>MIME タイプ<input value={metaForm.media_type} onChange={e=>setMetaForm({...metaForm,media_type:e.target.value})} placeholder="application/zip"/></label><label>ファイル取得元URL<input type="url" value={metaForm.source_url} onChange={e=>setMetaForm({...metaForm,source_url:e.target.value})}/></label><p className="muted">作者・作品名は作品レコードを更新するため、同じ作品に属するファイルの表示にも反映されます。</p><div className="modal-actions"><button type="button" className="secondary" onClick={()=>setMetadata(null)}>キャンセル</button><button className="primary" disabled={busy}>{busy?'保存中…':'メタデータを保存'}</button></div></form></Modal>}
  </>;
}
function Modal({title,close,children}:{title:string,close:()=>void,children:React.ReactNode}){return <div className="modal-backdrop" onMouseDown={e=>e.target===e.currentTarget&&close()}><div className="modal"><div className="modal-head"><h2>{title}</h2><button className="icon-btn" onClick={close}><X/></button></div>{children}</div></div>}
function Nodes({api,refresh,notify}:any){const d=useData(api,'/nodes',refresh);const[busy,setBusy]=useState(false);return <><Heading eyebrow="" title="保存ノード" desc="稼働状況と容量" actions={<button className="secondary" onClick={()=>api('/admin/placements/reconcile',{method:'POST'}).then(()=>notify('再調整を実行しました')).catch((e:Error)=>notify(e.message))}><RefreshCw size={16}/> 保存先を調整</button>}/><div className="stat-grid"><Stat label="登録ノード" value={(d.data.items||[]).length} sub="参加ノード数" icon={HardDrive}/><Stat label="オンライン" value={(d.data.items||[]).filter((x:Obj)=>x.status==='online').length} sub="稼働状況" icon={CheckCircle2} tone="green"/><Stat label="申告容量" value={bytes((d.data.items||[]).reduce((s:number,x:Obj)=>s+(x.capacity_bytes||0),0))} sub="参加者の設定値の合計" icon={Archive} tone="violet"/><Stat label="要確認" value={(d.data.items||[]).filter((x:Obj)=>x.status!=='online').length} sub="停止・劣化ノード" icon={AlertTriangle} tone="amber"/></div><section className="panel table-panel"><div className="panel-head"><div><h2>ノード一覧</h2><p>実容量と申告上限を表示</p></div></div><Table headers={['ノード','種別','状態','実容量 / 空き','申告上限','最終応答','エージェント']}>{d.loading?<tr><td colSpan={7} className="empty">読み込み中…</td></tr>:d.error?<tr><td colSpan={7} className="empty">{d.error}</td></tr>:(d.data.items||[]).map((x:Obj)=><tr key={x.id}><td><div className="file-cell"><div className="file-icon"><HardDrive size={17}/></div><div><b>{x.name||x.id}</b><small><code>{String(x.id).slice(0,8)}</code></small></div></div></td><td><span className="type-pill">{x.node_type||'—'}</span></td><td><Status value={x.status}/></td><td><b>{bytes(x.total_bytes??x.actual_capacity_bytes??x.capacity_bytes)}</b><small className="cell-sub">空き {bytes(x.available_bytes??x.free_bytes)}</small></td><td>{x.requested_capacity_bytes===0?'無制限':bytes(x.requested_capacity_bytes??x.capacity_bytes)}</td><td>{date(x.last_seen_at)}</td><td>{x.agent_version||'—'}</td></tr>)}</Table></section><div className="info-card"><AlertTriangle size={18}/><div><b>運用上の注意</b><p>開発用ノードは同じホスト上です。本番では別ホストに配置してください。</p></div></div></>}
function Tasks({api,refresh,notify,role}:any){const d=useData(api,'/tasks',refresh);const[filter,setFilter]=useState('all');const items=(d.data.items||[]).filter((x:Obj)=>filter==='all'||x.status===filter);async function retry(id:string){try{await api(`/tasks/${id}/retry`,{method:'POST'});notify('再試行を開始しました')}catch(e){notify((e as Error).message)}}return <><Heading eyebrow="" title="タスク" desc="処理状況と失敗理由" actions={<button className="secondary" onClick={()=>location.reload()}><RefreshCw size={16}/> 更新</button>}/><div className="filter-bar"><div className="filter-tabs">{[['all','すべて'],['queued','待機中'],['running','実行中'],['failed','失敗'],['succeeded','成功']].map(([v,l])=><button key={v} className={filter===v?'active':''} onClick={()=>setFilter(v)}>{l}</button>)}</div><span className="result-count">{items.length} 件</span></div><section className="panel table-panel"><Table headers={['タスクID','タスク種別','状態','優先度','試行回数','エラー / 結果','更新日時','操作']}>{d.loading?<tr><td colSpan={8} className="empty">読み込み中…</td></tr>:d.error?<tr><td colSpan={8} className="empty">{d.error}</td></tr>:items.length?items.map((x:Obj)=><tr key={x.id}><td><code>{String(x.id).slice(0,8)}</code></td><td>{x.task_type==='source_fetch'?'元ファイル取得（agent）':x.task_type==='store'?'保存':x.task_type==='replicate'?'複製':x.task_type==='delete'?'削除':x.task_type}</td><td><Status value={x.status}/></td><td><span className="priority">{x.priority??'—'}</span></td><td>{x.attempt_count??0} / 5</td><td className="error-cell">{x.last_error||'—'}</td><td>{date(x.updated_at)}</td><td>{role==='admin'&&x.status==='failed'&&<button className="small-btn" onClick={()=>retry(x.id)}>再試行</button>}</td></tr>):<tr><td colSpan={8} className="empty">タスクがありません</td></tr>}</Table></section><div className="info-card"><Clock3 size={18}/><div><b>再試行</b><p>失敗タスクは最大5回まで自動で再試行します。管理者は手動でも再試行できます。</p></div></div></>}
function Requests({api,refresh,notify,role,bumpRefresh}:any){
  const nav=useNavigate();
  const d=useData(api,'/public-requests?limit=100',refresh);
  const[filter,setFilter]=useState('all');
  const[selectedRequestIDs,setSelectedRequestIDs]=useState<string[]>([]);
  const[chosen,setChosen]=useState<Obj|null>(null);
  const[status,setStatus]=useState('in_review');
  const[reason,setReason]=useState('');
  const[preserveBusy,setPreserveBusy]=useState(false);
  const[preserveTerms,setPreserveTerms]=useState(false);
  const[preserveArtifactId,setPreserveArtifactId]=useState('');
  const[preserveCandidates,setPreserveCandidates]=useState<Obj[]>([]);
  const[preserveSelected,setPreserveSelected]=useState<string[]>([]);
  const[preserveStatus,setPreserveStatus]=useState('');
  const items=(d.data.items||[]).filter((x:Obj)=>filter==='all'||x.status===filter);
  function toggleRequest(id:string,checked:boolean){setSelectedRequestIDs(old=>checked?(old.includes(id)?old:[...old,id]):old.filter(x=>x!==id));}
  async function bulkReview(nextStatus:'accepted'|'rejected'){
    const selected=(d.data.items||[]).filter((x:Obj)=>selectedRequestIDs.includes(x.id));
    if(!selected.length){notify('申請を選択してください');return;}
    if(nextStatus==='accepted'&&role!=='admin'){notify('一括承認は管理者のみ利用できます');return;}
    if(nextStatus==='accepted'&&selected.some((x:Obj)=>!['deletion','takedown'].includes(x.request_type))){notify('一括承認は削除・公開停止申請のみ対応しています。保全などの申請は個別に確認してください');return;}
    if(selected.some((x:Obj)=>!['pending','in_review'].includes(x.status))){notify('未処理または確認中の申請を選択してください');return;}
    const reason=window.prompt(nextStatus==='accepted'?'一括承認の理由を入力してください':'一括却下の理由を入力してください');
    if(!reason||reason.trim().length<3)return;
    if(nextStatus==='accepted'&&!window.confirm(`選択した ${selected.length} 件の申請を承認します。削除申請は保存ノードへの削除タスクを登録します。続行しますか？`))return;
    let ok=0;const failures:string[]=[];
    for(const item of selected){try{await api(`/public-requests/${item.id}`,{method:'PATCH',body:JSON.stringify({status:nextStatus,reason:reason.trim()})});ok++;}catch(e){failures.push(`${String(item.id).slice(0,8)}: ${(e as Error).message}`)}}
    setSelectedRequestIDs([]);bumpRefresh?.();
    notify(`${ok} 件を処理しました${failures.length?`。失敗 ${failures.length} 件：${failures[0]}`:''}`);
  }
  async function submit(){
    if(!chosen)return;
    if(chosen.request_type==='deletion'&&status==='accepted'&&!window.confirm('削除申請を承認すると、公開停止に加えて保存ノードへの削除タスクが登録されます。続行しますか？'))return;
    try{
      const updated=await api(`/public-requests/${chosen.id}`,{method:'PATCH',body:JSON.stringify({status,reason})});
      let next:Obj=updated.request||{...chosen,status};
      if(role==='admin'&&next.request_type==='preservation'&&status==='in_review'&&next.source_url){
        const artifactID=await ensureArtifact(next);
        next={...next,artifact_id:artifactID,status:'in_review'};
        if(isIndividualDownload(next.source_url)&&!next.file_object_id){
          const file=await api(`/artifacts/${artifactID}/files`,{method:'POST',body:JSON.stringify({
            original_filename:'取得待ち',size_bytes:0,media_type:'application/octet-stream',
            source_url:next.source_url,redistribution_policy:'unknown'
          })});
          const linked=await api(`/public-requests/${next.id}`,{method:'PATCH',body:JSON.stringify({
            status:'in_review',reason:'確認中への変更時に保全対象ファイルを登録',artifact_id:artifactID,file_object_id:file.id
          })});
          next={...next,...(linked.request||{}),artifact_id:artifactID,file_object_id:file.id,status:'in_review'};
        }
        setPreserveArtifactId(artifactID);
      }
      setChosen(next);
      setReason('');
      notify(next.request_type==='deletion'&&status==='accepted'?'削除申請を承認し、保存ノードへの削除タスクを登録しました。':next.request_type==='takedown'&&status==='accepted'?'公開停止申請を承認し、ダウンロードを停止しました。':next.request_type==='preservation'&&status==='in_review'?(role==='admin'?(next.file_object_id?'確認中に変更し、保全対象ファイルを登録しました。取得開始は下のボタンから行ってください。':'確認中に変更しました。下のボタンからURLを調査してください。'):'確認中に変更しました。対象ファイルの登録・取得は管理者が実行してください。'):'申請を更新しました');
      bumpRefresh?.();
    }catch(e){notify((e as Error).message)}
  }
  async function ensureArtifact(req:Obj){
    if(req.artifact_id)return req.artifact_id as string;
    const artifact=await api('/artifacts',{method:'POST',body:JSON.stringify({
      title:req.title||'保全リクエスト',creator_name:req.creator_name||undefined,
      source_url:req.source_url,description:req.details||undefined
    })});
    const nextStatus=req.status==='pending'?'in_review':req.status;
    const updated=await api(`/public-requests/${req.id}`,{method:'PATCH',body:JSON.stringify({
      status:nextStatus,reason:'保全リクエストから作品情報を登録',artifact_id:artifact.id
    })});
    const next={...req,...(updated.request||{}),artifact_id:artifact.id,status:nextStatus};
    setChosen(next);
    setStatus(nextStatus);
    setPreserveArtifactId(artifact.id);
    bumpRefresh?.();
    return artifact.id as string;
  }
  function isIndividualDownload(raw:string){
    try{const u=new URL(raw);return u.pathname.toLowerCase().includes('/download/')||(u.hostname.toLowerCase().startsWith('dl')&&u.hostname.toLowerCase().includes('getuploader.com')&&u.pathname.toLowerCase().includes('/g/'))}catch{return false}
  }
  async function startPreservation(){
    if(!chosen||chosen.request_type!=='preservation'||!chosen.source_url||!preserveTerms)return;
    setPreserveBusy(true);setPreserveStatus('保全対象を準備しています…');setPreserveCandidates([]);setPreserveSelected([]);
    try{
      const artifactID=await ensureArtifact(chosen);
      if(isIndividualDownload(chosen.source_url)){
        let fileID=chosen.file_object_id as string|undefined;
        if(!fileID){
          const file=await api(`/artifacts/${artifactID}/files`,{method:'POST',body:JSON.stringify({
            original_filename:'取得待ち',size_bytes:0,media_type:'application/octet-stream',
            source_url:chosen.source_url,redistribution_policy:'unknown'
          })});
          fileID=file.id;
          const reqStatus=chosen.status==='pending'?'in_review':chosen.status;
          const updated=await api(`/public-requests/${chosen.id}`,{method:'PATCH',body:JSON.stringify({
            status:reqStatus,reason:'保全対象ファイルを登録',artifact_id:artifactID,file_object_id:file.id
          })});
          setChosen({...chosen,...(updated.request||{}),artifact_id:artifactID,file_object_id:file.id,status:reqStatus});
        }
        setPreserveStatus('agent に取得タスクを登録しています…');
        try{
          const result=await api(`/files/${fileID}/ingest-source`,{method:'POST',body:JSON.stringify({source_url:chosen.source_url,accept_terms:true,priority:3})});
          const fetchTask=result.source_fetch_task;
          setPreserveStatus(fetchTask?`取得タスクを ${fetchTask.node_name||'agent'} に登録しました。agent は設定された間隔を空けて取得し、成功後に保存タスクを作成します。`:result.message||'取得タスクを登録しました。');
          notify('取得タスクを agent に登録しました。保存タスクは取得成功後に作成されます。');
        }catch(e){setPreserveStatus(`ファイルは登録済みですが、取得タスクの登録に失敗しました: ${(e as Error).message}`)}
        bumpRefresh?.();
      }else{
        setPreserveStatus('一覧ページを調べています…');
        const result=await api(`/artifacts/${artifactID}/discover-source-files`,{method:'POST',body:JSON.stringify({listing_url:chosen.source_url,accept_terms:true})});
        const found=(result.items||[]) as Obj[];
        setPreserveCandidates(found);
        setPreserveSelected(found.map((x:Obj)=>x.url));
        setPreserveStatus(`${result.pages_scanned??0} ページを調査し、${found.length} 件の候補を見つけました。取得するファイルを選択してください。`);
        if(found.length===0)setPreserveStatus('一覧ページから取得候補を見つけられませんでした。URLが一覧ページか、管理画面「作品・ファイル」から確認してください。');
      }
    }catch(e){setPreserveStatus(`保全準備に失敗しました: ${(e as Error).message}`)}
    finally{setPreserveBusy(false)}
  }
  async function ingestSelectedCandidates(){
    if(!chosen||!preserveArtifactId||preserveSelected.length===0)return;
    const selected=preserveCandidates.filter((x:Obj)=>preserveSelected.includes(x.url));
    setPreserveBusy(true);let queuedCount=0;let storageTaskCount=0;let skipped=0;const errors:string[]=[];let linkedFileID=chosen.file_object_id as string|undefined;
    try{
      const existingResult=await api(`/files?artifact_id=${encodeURIComponent(preserveArtifactId)}&limit=100`);
      const existingFiles:Obj[]=[...(existingResult.items||[])];
      for(let i=0;i<selected.length;i++){
        const item=selected[i];setPreserveStatus(`取得タスク登録中 ${i+1}/${selected.length}: ${item.label||item.url}`);
        try{
          const existingFile=existingFiles.find((x:Obj)=>x.source_url===item.url);
          if(existingFile?.preservation_status==='verified'){skipped++;continue;}
          let file:Obj;
          if(existingFile){
            file=existingFile;
          }else{
            const createdFile:Obj=await api(`/artifacts/${preserveArtifactId}/files`,{method:'POST',body:JSON.stringify({
              original_filename:(item.label||'取得待ち').slice(0,500),size_bytes:0,media_type:'application/octet-stream',
              source_url:item.url,redistribution_policy:'unknown'
            })});
            if(!createdFile || typeof createdFile.id!=='string' || !createdFile.id){
              throw new Error('保全対象ファイルの登録結果にファイルIDがありません');
            }
            existingFiles.push(createdFile);
            file=createdFile;
          }
          if(!linkedFileID){
            const reqStatus=chosen.status==='pending'?'in_review':chosen.status;
            const updated=await api(`/public-requests/${chosen.id}`,{method:'PATCH',body:JSON.stringify({
              status:reqStatus,reason:'保全対象ファイルを登録',artifact_id:preserveArtifactId,file_object_id:file.id
            })});
            linkedFileID=file.id;setChosen({...chosen,...(updated.request||{}),artifact_id:preserveArtifactId,file_object_id:file.id,status:reqStatus});
          }
          const result=await api(`/files/${file.id}/ingest-source`,{method:'POST',body:JSON.stringify({source_url:item.url,accept_terms:true,priority:3})});
          if(result.source_fetch_task)queuedCount++;
          storageTaskCount+=Array.isArray(result.tasks)?result.tasks.length:0;
        }catch(e){errors.push(`${item.label||item.url}: ${(e as Error).message}`)}
      }
    }catch(e){errors.push((e as Error).message)}
    setPreserveBusy(false);setPreserveSelected([]);setPreserveCandidates([]);
    setPreserveStatus(`処理終了: 取得タスク ${queuedCount} 件を agent に登録、保存タスク ${storageTaskCount} 件作成、保全済みスキップ ${skipped} 件、失敗 ${errors.length} 件。${errors.length?`最初の失敗: ${errors[0]}`:'保存タスクは各 agent の取得成功後に作成されます。'}`);
    notify(`取得タスク ${queuedCount} 件登録、保全済み ${skipped} 件スキップ、失敗 ${errors.length} 件`);bumpRefresh?.();
  }
  return <>
    <Heading eyebrow="" title="申請" desc="保全・削除・公開停止の申請"/>
    <div className="notice amber-notice"><div className="notice-icon"><AlertTriangle size={19}/></div><div><b>公開停止を優先</b><p>公開停止は承認時に即時反映されます。削除申請は承認するとノードへの削除タスクを登録し、全保存先の削除成功後に一覧から非表示になります。保全リクエストは個別URL・一覧URLの候補を確認してから取得を開始します。</p></div></div>
    <div className="filter-bar"><div className="filter-tabs">{[['all','すべて'],['pending','未処理'],['in_review','確認中'],['accepted','承認'],['rejected','却下'],['closed','終了']].map(([v,l])=><button key={v} className={filter===v?'active':''} onClick={()=>{setFilter(v);setSelectedRequestIDs([])}}>{l}</button>)}</div></div>
    {selectedRequestIDs.length>0&&role!=='viewer'&&<section className="bulk-toolbar panel"><div className="bulk-toolbar-title"><b>{selectedRequestIDs.length} 件の申請を選択中</b><button className="small-btn" onClick={()=>setSelectedRequestIDs([])}>選択解除</button></div><div className="bulk-toolbar-controls">{role==='admin'&&<button className="primary" onClick={()=>bulkReview('accepted')}>公開停止・削除を一括承認</button>}<button className="secondary" onClick={()=>bulkReview('rejected')}>一括却下</button></div></section>}
    <section className="panel table-panel"><Table headers={['','種別','申請者','対象ファイル','状態','受付日時','操作']}>
      {d.loading?<tr><td colSpan={7} className="empty">読み込み中…</td></tr>:d.error?<tr><td colSpan={7} className="empty">{d.error}</td></tr>:items.length?items.map((x:Obj)=><tr key={x.id}><td><input type="checkbox" aria-label="申請を選択" checked={selectedRequestIDs.includes(x.id)} disabled={role==='viewer'} onChange={e=>toggleRequest(x.id,e.target.checked)}/></td><td>{x.request_type==='takedown'?'公開停止':x.request_type==='deletion'?'削除':x.request_type==='preservation'?'保全リクエスト':'移設先情報'}</td><td>{x.submitter_name||'匿名'}</td><td><div className="request-file-cell"><b>{x.linked_file_name||x.linked_artifact_title||(x.file_object_id?String(x.file_object_id).slice(0,10):x.artifact_id?String(x.artifact_id).slice(0,10)+'（作品）':x.request_type==='preservation'&&x.source_url?(isIndividualDownload(x.source_url)?'個別URLあり・未登録':'一覧URLあり・候補未登録'):'未指定')}</b>{(x.file_object_id||x.artifact_id)&&<button className="small-btn" onClick={()=>nav(`/admin/review?tab=files&${x.file_object_id?'file_id='+encodeURIComponent(x.file_object_id):'artifact_id='+encodeURIComponent(x.artifact_id)}`)}>ファイルを開く</button>}</div></td><td><Status value={x.status}/></td><td>{date(x.created_at)}</td><td><button className="small-btn" onClick={()=>{setChosen(x);setStatus(x.status==='pending'?'in_review':x.status);setReason('');setPreserveTerms(false);setPreserveArtifactId(x.artifact_id||'');setPreserveCandidates([]);setPreserveSelected([]);setPreserveStatus('')}}>確認</button></td></tr>):<tr><td colSpan={7} className="empty">申請がありません</td></tr>}
    </Table></section>
    {chosen&&<Modal title="申請内容" close={()=>{setChosen(null);setPreserveCandidates([]);setPreserveStatus('')}}>
      {(chosen.file_object_id||chosen.artifact_id)&&<div className="request-file-shortcut"><div><b>{chosen.linked_file_name||chosen.linked_artifact_title||'関連ファイル'}</b><small>申請から対象ファイル・作品を直接参照できます。</small></div><button className="secondary" onClick={()=>nav(`/admin/review?tab=files&${chosen.file_object_id?'file_id='+encodeURIComponent(chosen.file_object_id):'artifact_id='+encodeURIComponent(chosen.artifact_id)}`)}>ファイル一覧で開く</button></div>}
      <div className="request-detail"><div className="request-type">{chosen.request_type==='preservation'?'保全リクエスト':chosen.request_type==='takedown'?'公開停止':chosen.request_type==='deletion'?'削除':'移設先情報'}</div>{chosen.title&&<p><b>{chosen.title}</b></p>}{chosen.creator_name&&<p>作者・配布者: {chosen.creator_name}</p>}<p>{chosen.details}</p><dl><dt>申請ID</dt><dd><code>{chosen.id}</code></dd><dt>対象ファイル</dt><dd>{chosen.file_object_id?<code>{chosen.file_object_id}</code>:chosen.artifact_id?<span>作品登録済み（ファイル未登録）<br/><code>{chosen.artifact_id}</code></span>:chosen.request_type==='preservation'&&chosen.source_url?'未登録（保全元URLはあります）':'—'}</dd><dt>連絡先</dt><dd>{chosen.contact||'未記入'}</dd><dt>保全元URL</dt><dd>{chosen.source_url?<a href={chosen.source_url} target="_blank" rel="noreferrer">{chosen.source_url} <ExternalLink size={13}/></a>:'—'}</dd><dt>移設先URL</dt><dd>{chosen.external_url?<a href={chosen.external_url} target="_blank" rel="noreferrer">{chosen.external_url} <ExternalLink size={13}/></a>:'—'}</dd><dt>受付日時</dt><dd>{date(chosen.created_at)}</dd></dl></div>
      {role!=='viewer'&&<div className="form-stack"><label>対応ステータス<select value={status} onChange={e=>setStatus(e.target.value)}><option value="in_review">確認中</option><option value="accepted" disabled={chosen.request_type==='deletion'&&role!=='admin'}>承認</option><option value="rejected">却下</option><option value="closed">終了</option></select></label><label>判断理由 *<textarea rows={3} minLength={3} value={reason} onChange={e=>setReason(e.target.value)} placeholder="理由を入力してください"/></label><div className="modal-actions"><button className="secondary" onClick={()=>setChosen(null)}>閉じる</button><button className="primary" disabled={reason.trim().length<3} onClick={submit}>ステータスを保存</button></div></div>}
      {role==='admin'&&chosen.request_type==='preservation'&&chosen.source_url&&['pending','in_review'].includes(chosen.status)&&<div className="preservation-workflow form-stack"><div className="form-divider"/><h3>このリクエストから保全する</h3><p className="muted">「確認中」は申請の状態です。個別URLは確認中への変更時に対象ファイルを登録します。一覧URLは候補を調査し、選択したファイルごとに取得タスクを登録します。agent が設定間隔を空けて取得し、成功後に保存タスクを作成します。公開設定は有効にしません。</p><label className="check-label"><input type="checkbox" checked={preserveTerms} onChange={e=>setPreserveTerms(e.target.checked)}/> uploader.jp の利用規約を確認しました</label><button className="primary" type="button" disabled={!preserveTerms||preserveBusy||!['pending','in_review'].includes(chosen.status)} onClick={startPreservation}>{preserveBusy?'処理中…':'URLを調査・取得タスク登録'}</button>
        {preserveStatus&&<div className="info-card"><div><b>処理結果</b><p>{preserveStatus}</p></div></div>}
        {preserveCandidates.length>0&&<><div className="candidate-list">{preserveCandidates.map((item:Obj)=><label className="candidate-row" key={item.url}><input type="checkbox" checked={preserveSelected.includes(item.url)} onChange={e=>setPreserveSelected(old=>e.target.checked?[...old,item.url]:old.filter(x=>x!==item.url))}/><span><b>{item.label||'ファイル候補'}</b><small>{item.url}</small></span></label>)}</div><div className="modal-actions"><button type="button" className="secondary" onClick={()=>setPreserveSelected(preserveCandidates.map((x:Obj)=>x.url))}>すべて選択</button><button type="button" className="primary" disabled={preserveBusy||preserveSelected.length===0} onClick={ingestSelectedCandidates}>選択した {preserveSelected.length} 件を保全</button></div></>}
      </div>}
    </Modal>}
  </>;
}
function UsersPage({api,refresh,notify,role}:any){const d=useData(api,'/admin/users',refresh);const[modal,setModal]=useState(false);const[form,setForm]=useState({email:'',display_name:'',password:'',role:'reviewer'});async function create(e:React.FormEvent){e.preventDefault();try{await api('/admin/users',{method:'POST',body:JSON.stringify(form)});notify('ユーザーを作成しました');setModal(false)}catch(e){notify((e as Error).message)}}return <><Heading eyebrow="" title="ユーザー" desc="ユーザーと権限" actions={role==='admin'&&<button className="primary" onClick={()=>setModal(true)}><Plus size={16}/> ユーザーを追加</button>}/>{role!=='admin'?<div className="info-card"><LockKeyhole/><div><b>管理者のみ</b><p>ユーザー管理は管理者のみ利用できます。</p></div></div>:<section className="panel table-panel"><Table headers={['ユーザー','メールアドレス','ロール','状態','作成日時']}>{d.loading?<tr><td colSpan={5} className="empty">読み込み中…</td></tr>:d.error?<tr><td colSpan={5} className="empty">{d.error}</td></tr>:(d.data.items||[]).map((x:Obj)=><tr key={x.id}><td><div className="file-cell"><div className="avatar">{(x.display_name||x.email).slice(0,1).toUpperCase()}</div><b>{x.display_name}</b></div></td><td>{x.email}</td><td><span className="type-pill">{x.role}</span></td><td><Status value={x.disabled_at?'offline':'online'}/></td><td>{date(x.created_at)}</td></tr>)}</Table></section>}{modal&&<Modal title="ユーザーを作成" close={()=>setModal(false)}><form className="form-stack" onSubmit={create}><label>表示名<input required value={form.display_name} onChange={e=>setForm({...form,display_name:e.target.value})}/></label><label>メールアドレス<input required type="email" value={form.email} onChange={e=>setForm({...form,email:e.target.value})}/></label><label>初期パスワード（12文字以上）<input required minLength={12} type="password" value={form.password} onChange={e=>setForm({...form,password:e.target.value})}/></label><label>ロール<select value={form.role} onChange={e=>setForm({...form,role:e.target.value})}><option value="reviewer">確認担当者</option><option value="viewer">閲覧者</option><option value="admin">管理者</option></select></label><div className="modal-actions"><button type="button" className="secondary" onClick={()=>setModal(false)}>キャンセル</button><button className="primary">作成</button></div></form></Modal>}</>}
function Audit({api,refresh}:any){const d=useData(api,'/admin/audit-logs',refresh);return <><Heading eyebrow="" title="操作履歴" desc="管理操作の履歴"/><section className="panel table-panel"><Table headers={['日時','実行主体','対象','操作','結果','理由']}>{d.loading?<tr><td colSpan={6} className="empty">読み込み中…</td></tr>:d.error?<tr><td colSpan={6} className="empty">{d.error}</td></tr>:(d.data.items||[]).map((x:Obj)=><tr key={x.id}><td>{date(x.created_at)}</td><td>{x.actor_email||x.actor_id||'system'}</td><td>{x.target_type} <code>{String(x.target_id||'').slice(0,8)}</code></td><td>{x.action}</td><td><Status value={x.result||x.status||'success'}/></td><td>{x.reason||x.details||'—'}</td></tr>)}</Table></section></>}

export default App;
