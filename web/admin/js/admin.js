/* 归来小说CMS - Admin Panel v2.0 */
const { createApp, ref, reactive, computed, onMounted, nextTick } = Vue;
const { createRouter, createWebHashHistory } = VueRouter;
const { ElMessage, ElMessageBox } = ElementPlus;
const Icons = window.ElementPlusIconsVue || {};

// API
const API = axios.create({ baseURL: '/api/v1' });
API.interceptors.request.use(c => {
  const t = localStorage.getItem('atok');
  if (t) c.headers.Authorization = 'Bearer ' + t;
  return c;
});
API.interceptors.response.use(r => r, e => {
  if (e.response?.status === 401) { localStorage.removeItem('atok'); location.reload(); }
  return Promise.reject(e);
});

const atok = () => localStorage.getItem('atok') || '';
const setAtok = (t, u) => { localStorage.setItem('atok', t); localStorage.setItem('auser', u); };

// ── Login Component ─────────────────────────────────────────────────────
const LoginPage = {
  template: `
  <div class="login-container">
    <div class="login-card">
      <h1>📚 归来小说CMS</h1>
      <p class="sub">管理后台</p>
      <el-tabs v-model="tab" class="login-tabs">
        <el-tab-pane label="登录" name="login">
          <el-form @submit.prevent="login">
            <el-form-item><el-input v-model="lf.username" placeholder="用户名" size="large"/></el-form-item>
            <el-form-item><el-input v-model="lf.password" type="password" placeholder="密码" show-password size="large"/></el-form-item>
            <el-button type="primary" @click="login" :loading="loading" size="large" style="width:100%">登 录</el-button>
          </el-form>
        </el-tab-pane>
        <el-tab-pane label="注册" name="register">
          <el-form @submit.prevent="register">
            <el-form-item><el-input v-model="rf.username" placeholder="用户名" size="large"/></el-form-item>
            <el-form-item><el-input v-model="rf.email" placeholder="邮箱(选填)" size="large"/></el-form-item>
            <el-form-item><el-input v-model="rf.password" type="password" placeholder="密码(≥8位)" show-password size="large"/></el-form-item>
            <el-button type="success" @click="register" :loading="rloading" size="large" style="width:100%">注 册</el-button>
          </el-form>
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>`,
  setup() {
    const tab = ref('login'); const loading = ref(false); const rloading = ref(false);
    const lf = reactive({ username: '', password: '' });
    const rf = reactive({ username: '', password: '', email: '' });
    const login = async () => {
      loading.value = true;
      try { const r = await axios.post('/api/v1/login', lf); setAtok(r.data.access_token, lf.username); location.reload(); }
      catch(e) { ElMessage.error(e.response?.data?.error || '登录失败'); }
      loading.value = false;
    };
    const register = async () => {
      if (rf.password.length < 8) return ElMessage.warning('密码至少8位');
      rloading.value = true;
      try { await axios.post('/api/v1/register', rf); ElMessage.success('注册成功，请登录'); tab.value='login'; lf.username=rf.username; }
      catch(e) { ElMessage.error(e.response?.data?.error || '注册失败'); }
      rloading.value = false;
    };
    return { tab, loading, rloading, lf, rf, login, register };
  }
};

// ── Main Layout ──────────────────────────────────────────────────────────
const MainLayout = {
  template: `
  <div style="display:flex;width:100%">
    <div class="sidebar">
      <div class="logo"><span>📚</span> 归来CMS</div>
      <el-menu :default-active="route.path" router background-color="transparent" text-color="#c0c4cc" active-text-color="#fff">
        <el-menu-item index="/dashboard"><el-icon><Odometer/></el-icon><span>控制台</span></el-menu-item>
        <el-menu-item index="/novels"><el-icon><Reading/></el-icon><span>小说管理</span></el-menu-item>
        <el-menu-item index="/categories"><el-icon><Collection/></el-icon><span>分类管理</span></el-menu-item>
        <el-menu-item index="/sites"><el-icon><Monitor/></el-icon><span>站点管理</span></el-menu-item>
        <el-menu-item index="/link-rings"><el-icon><Connection/></el-icon><span>链轮管理</span></el-menu-item>
        <el-menu-item index="/crawler"><el-icon><Download/></el-icon><span>采集任务</span></el-menu-item>
        <el-menu-item index="/cache"><el-icon><Setting/></el-icon><span>缓存运维</span></el-menu-item>
      </el-menu>
    </div>
    <div class="main-area">
      <div class="header">
        <span class="title">{{ pageTitle }}</span>
        <div class="user-info">
          <span>{{ user }}</span>
          <el-button link type="danger" @click="logout">退出</el-button>
        </div>
      </div>
      <div class="content"><router-view/></div>
    </div>
  </div>`,
  setup() {
    const route = VueRouter.useRoute();
    const pageTitle = computed(() => route.meta.title || '');
    const user = localStorage.getItem('auser') || '';
    const logout = () => { localStorage.removeItem('atok'); localStorage.removeItem('auser'); location.reload(); };
    return { route, pageTitle, user, logout };
  }
};

// ── Dashboard ────────────────────────────────────────────────────────────
const Dashboard = {
  template: `
  <div>
    <div class="page-header"><h2>📊 控制台</h2><p class="desc">系统运行概览</p></div>
    <div class="stat-grid">
      <div class="stat-card" v-for="s in stats" :key="s.l">
        <div class="icon" :class="s.c"><el-icon :size="22"><component :is="s.i"/></el-icon></div>
        <div><div class="value">{{ s.v }}</div><div class="label">{{ s.l }}</div></div>
      </div>
    </div>
    <div class="card">
      <h3 style="margin-bottom:12px">系统状态</h3>
      <el-descriptions :column="2" border>
        <el-descriptions-item label="服务状态">{{ health.status || '-' }}</el-descriptions-item>
        <el-descriptions-item label="版本">{{ health.version || '-' }}</el-descriptions-item>
        <el-descriptions-item label="数据库">{{ health.database || '-' }}</el-descriptions-item>
        <el-descriptions-item label="内存">{{ mem.alloc_mb||0 }} MB</el-descriptions-item>
        <el-descriptions-item label="Goroutines">{{ mem.goroutines||0 }}</el-descriptions-item>
        <el-descriptions-item label="GC">{{ mem.num_gc||0 }}</el-descriptions-item>
      </el-descriptions>
    </div>
  </div>`,
  setup() {
    const stats = ref([]); const health = ref({}); const mem = ref({});
    onMounted(async () => {
      axios.get('/health').then(r=>health.value=r.data).catch(()=>{});
      API.get('/crawler/stats').then(r=>{ const d=r.data; stats.value=[
        {l:'小说总数',v:d.novels||0,i:'Reading',c:'blue'},
        {l:'章节总数',v:d.chapters||0,i:'Collection',c:'green'},
        {l:'采集任务',v:d.tasks_total||0,i:'Download',c:'orange'},
        {l:'待处理',v:d.tasks_pending||0,i:'Clock',c:'red'},
      ];}).catch(()=>{});
      API.get('/cache/health').then(r=>{mem.value=r.data.memory||{}}).catch(()=>{});
    });
    return { stats, health, mem };
  }
};

// ── Novels ───────────────────────────────────────────────────────────────
const Novels = {
  template: `
  <div>
    <div class="page-header"><h2>📚 小说管理</h2></div>
    <div class="card">
      <div class="toolbar">
        <div class="toolbar-left">
          <el-input v-model="s" placeholder="搜索..." clearable style="width:180px" @keyup.enter="load"/>
          <el-select v-model="st" placeholder="状态" clearable style="width:110px" @change="load"><el-option label="连载中" value="ongoing"/><el-option label="已完结" value="completed"/><el-option label="暂停" value="hiatus"/></el-select>
          <el-button @click="load">搜索</el-button>
        </div>
        <el-button type="primary" @click="create">+ 新建</el-button>
      </div>
      <el-table :data="items" stripe v-loading="ld" @row-click="(r)=>$router.push('/novels/'+r.id)" style="cursor:pointer">
        <el-table-column prop="title" label="书名" min-width="180"/>
        <el-table-column prop="author" label="作者" width="120"/>
        <el-table-column label="状态" width="80"><template #d="s"><span :class="'status-tag '+s.row.status">{{sm[s.row.status]||s.row.status}}</span></template></el-table-column>
        <el-table-column prop="total_chapters" label="章节" width="70"/>
        <el-table-column label="操作" width="150" fixed="right"><template #d="s">
          <el-button link type="primary" size="small" @click.stop="$router.push('/novels/'+s.row.id)">详情</el-button>
          <el-button link type="danger" size="small" @click.stop="del(s.row.id)">删除</el-button>
        </template></el-table-column>
      </el-table>
      <div style="margin-top:14px;text-align:right"><el-pagination background layout="total,prev,next" :total="total" :page-size="20" v-model:current-page="pg" @current-change="load"/></div>
    </div>
    <el-dialog v-model="dlg" title="新建小说" width="500px">
      <el-form :model="f" label-width="70px">
        <el-form-item label="书名"><el-input v-model="f.title"/></el-form-item>
        <el-form-item label="作者"><el-input v-model="f.author"/></el-form-item>
        <el-form-item label="简介"><el-input v-model="f.description" type="textarea" :rows="3"/></el-form-item>
        <el-form-item label="源URL"><el-input v-model="f.source_url"/></el-form-item>
        <el-form-item label="源名"><el-input v-model="f.source_name"/></el-form-item>
        <el-form-item label="状态"><el-select v-model="f.status"><el-option label="连载中" value="ongoing"/><el-option label="已完结" value="completed"/><el-option label="暂停" value="hiatus"/></el-select></el-form-item>
      </el-form>
      <template #footer><el-button @click="dlg=false">取消</el-button><el-button type="primary" @click="save">创建</el-button></template>
    </el-dialog>
  </div>`,
  setup() {
    const items=ref([]); const total=ref(0); const pg=ref(1); const ld=ref(false); const s=ref(''); const st=ref('');
    const dlg=ref(false); const f=reactive({title:'',author:'',description:'',source_url:'',source_name:'',status:'ongoing'});
    const sm={ongoing:'连载中',completed:'已完结',hiatus:'暂停'};
    const load=async()=>{ ld.value=true; try { const r=await API.get('/novels',{params:{page:pg.value,search:s.value,status:st.value}}); items.value=r.data.items; total.value=r.data.total; }catch(e){} ld.value=false; };
    const save=async()=>{ try { await API.post('/novels',f); ElMessage.success('创建成功'); dlg.value=false; load(); }catch(e){ElMessage.error('创建失败');} };
    const del=async(id)=>{ try { await ElMessageBox.confirm('确定删除？','警告',{type:'warning'}); await API.delete('/novels/'+id); ElMessage.success('已删除'); load(); }catch(e){} };
    const create=()=>{ Object.assign(f,{title:'',author:'',description:'',source_url:'',source_name:'',status:'ongoing'}); dlg.value=true; };
    onMounted(load);
    return { items,total,pg,ld,s,st,dlg,f,sm,create,save,del,load };
  }
};

// ── Novel Detail (with chapters) ────────────────────────────────────────
const NovelDetail = {
  props: ['id'],
  template: `
  <div>
    <div class="page-header"><h2>📖 {{ n.title || '...' }}</h2><p><el-button link @click="$router.push('/novels')">← 返回</el-button></p></div>
    <div class="card" v-if="n.id">
      <el-descriptions :column="2" border>
        <el-descriptions-item label="书名">{{n.title}}</el-descriptions-item>
        <el-descriptions-item label="作者">{{n.author||'-'}}</el-descriptions-item>
        <el-descriptions-item label="状态"><span :class="'status-tag '+n.status">{{sm[n.status]}}</span></el-descriptions-item>
        <el-descriptions-item label="章节数">{{n.total_chapters}}</el-descriptions-item>
        <el-descriptions-item label="来源">{{n.source_name||'-'}}</el-descriptions-item>
        <el-descriptions-item label="创建">{{n.created_at}}</el-descriptions-item>
        <el-descriptions-item label="简介" :span="2">{{n.description||'暂无'}}</el-descriptions-item>
      </el-descriptions>
      <div style="margin-top:14px"><el-button type="primary" @click="addCh">+ 新建章节</el-button></div>
    </div>
    <div class="card" style="margin-top:16px">
      <h3 style="margin-bottom:12px">章节列表</h3>
      <el-table :data="chs" stripe v-loading="cl" max-height="500">
        <el-table-column prop="sort_order" label="#" width="60"/>
        <el-table-column prop="title" label="标题" min-width="200"/>
        <el-table-column prop="word_count" label="字数" width="80"/>
        <el-table-column label="操作" width="180"><template #d="r">
          <el-button link type="primary" size="small" @click="editCh(r.row)">编辑</el-button>
          <el-button link type="danger" size="small" @click="delCh(r.row.id)">删除</el-button>
        </template></el-table-column>
      </el-table>
      <div style="margin-top:12px;text-align:right"><el-pagination background layout="prev,next" :total="n.total_chapters" :page-size="50" v-model:current-page="cp" @current-change="loadCh"/></div>
    </div>
    <el-dialog v-model="ach" title="新建章节" width="650px">
      <el-form :model="cf" label-width="70px">
        <el-form-item label="标题"><el-input v-model="cf.title"/></el-form-item>
        <el-form-item label="内容"><el-input v-model="cf.content" type="textarea" :rows="14"/></el-form-item>
        <el-form-item label="卷"><el-input v-model="cf.volume"/></el-form-item>
      </el-form>
      <template #footer><el-button @click="ach=false">取消</el-button><el-button type="primary" @click="saveCh">创建</el-button></template>
    </el-dialog>
    <el-dialog v-model="ech" title="编辑章节" width="650px">
      <el-form :model="ef" label-width="70px">
        <el-form-item label="标题"><el-input v-model="ef.title"/></el-form-item>
        <el-form-item label="内容"><el-input v-model="ef.content" type="textarea" :rows="14"/></el-form-item>
      </el-form>
      <template #footer><el-button @click="ech=false">取消</el-button><el-button type="primary" @click="updCh">保存</el-button></template>
    </el-dialog>
  </div>`,
  setup(props) {
    const n=ref({}); const chs=ref([]); const cp=ref(1); const cl=ref(false);
    const ach=ref(false); const ech=ref(false);
    const cf=reactive({title:'',content:'',volume:''});
    const ef=reactive({id:'',title:'',content:''});
    const sm={ongoing:'连载中',completed:'已完结',hiatus:'暂停'};
    const loadN=async()=>{ try { const r=await API.get('/novels/'+props.id); n.value=r.data; }catch(e){ElMessage.error('不存在');$router.push('/novels');} };
    const loadCh=async()=>{ cl.value=true; try { const r=await API.get('/novels/'+props.id+'/chapters',{params:{page:cp.value,size:50}}); chs.value=r.data.items; }catch(e){} cl.value=false; };
    const addCh=()=>{ cf.title='';cf.content='';cf.volume=''; ach.value=true; };
    const saveCh=async()=>{ try { await API.post('/novels/'+props.id+'/chapters',cf); ElMessage.success('创建成功'); ach.value=false; loadCh(); loadN(); }catch(e){ElMessage.error('创建失败');} };
    const editCh=async(ch)=>{ ef.id=ch.id; ef.title=ch.title; ech.value=true; try { const r=await API.get('/novels/'+props.id+'/chapters/'+ch.id); ef.content=r.data.content||''; }catch(e){} };
    const updCh=async()=>{ try { await API.put('/novels/'+props.id+'/chapters/'+ef.id,{title:ef.title,content:ef.content}); ElMessage.success('已保存'); ech.value=false; loadCh(); }catch(e){ElMessage.error('保存失败');} };
    const delCh=async(id)=>{ try { await ElMessageBox.confirm('确定删除？','警告',{type:'warning'}); await API.delete('/novels/'+props.id+'/chapters/'+id); ElMessage.success('已删除'); loadCh(); loadN(); }catch(e){} };
    onMounted(()=>{loadN();loadCh();});
    return { n,chs,cp,cl,ach,ech,cf,ef,sm,addCh,saveCh,editCh,updCh,delCh,loadCh };
  }
};

// ── Categories ───────────────────────────────────────────────────────────
const Categories = {
  template: `
  <div>
    <div class="page-header"><h2>🏷️ 分类管理</h2></div>
    <div class="card">
      <div class="toolbar"><div></div><el-button type="primary" @click="add">+ 新建</el-button></div>
      <el-table :data="items" stripe v-loading="ld">
        <el-table-column prop="id" label="ID" width="60"/><el-table-column prop="name" label="名称" width="150"/>
        <el-table-column prop="slug" label="标识" width="150"/><el-table-column prop="sort_order" label="排序" width="80"/>
        <el-table-column label="操作" width="160"><template #d="r">
          <el-button link type="primary" size="small" @click="edit(r.row)">编辑</el-button>
          <el-button link type="danger" size="small" @click="del(r.row.id)">删除</el-button>
        </template></el-table-column>
      </el-table>
    </div>
    <el-dialog v-model="dlg" :title="editing?'编辑':'新建'" width="400px">
      <el-form :model="f"><el-form-item label="名称"><el-input v-model="f.name"/></el-form-item><el-form-item label="标识"><el-input v-model="f.slug"/></el-form-item><el-form-item label="排序"><el-input-number v-model="f.sort_order"/></el-form-item></el-form>
      <template #footer><el-button @click="dlg=false">取消</el-button><el-button type="primary" @click="save">{{editing?'保存':'创建'}}</el-button></template>
    </el-dialog>
  </div>`,
  setup() {
    const items=ref([]); const ld=ref(false); const dlg=ref(false); const editing=ref(false);
    const f=reactive({id:'',name:'',slug:'',sort_order:0});
    const load=async()=>{ ld.value=true; try { const r=await API.get('/categories'); items.value=r.data; }catch(e){} ld.value=false; };
    const add=()=>{ editing.value=false; f.id='';f.name='';f.slug='';f.sort_order=0; dlg.value=true; };
    const edit=(c)=>{ editing.value=true; f.id=c.id;f.name=c.name;f.slug=c.slug;f.sort_order=c.sort_order; dlg.value=true; };
    const save=async()=>{
      try {
        if(editing.value) await API.put('/categories/'+f.id,{name:f.name,slug:f.slug,sort_order:f.sort_order});
        else await API.post('/categories',{name:f.name,slug:f.slug,sort_order:f.sort_order});
        ElMessage.success(editing.value?'已保存':'创建成功'); dlg.value=false; load();
      }catch(e){ElMessage.error('操作失败');}
    };
    const del=async(id)=>{ try { await ElMessageBox.confirm('确定删除？','警告',{type:'warning'}); await API.delete('/categories/'+id); ElMessage.success('已删除'); load(); }catch(e){} };
    onMounted(load);
    return { items,ld,dlg,editing,f,add,edit,save,del };
  }
};

// ── Sites ────────────────────────────────────────────────────────────────
const Sites = {
  template: `
  <div>
    <div class="page-header"><h2>🌐 站点管理</h2></div>
    <div class="card">
      <div class="toolbar"><div></div><el-button type="primary" @click="add">+ 新建</el-button></div>
      <el-table :data="items" stripe v-loading="ld">
        <el-table-column prop="name" label="名称" width="150"/><el-table-column prop="domain" label="域名" width="200"/>
        <el-table-column prop="template" label="模板" width="100"/><el-table-column prop="language" label="语言" width="80"/>
        <el-table-column label="状态" width="80"><template #d="r"><el-tag :type="r.row.is_active?'success':'danger'" size="small">{{r.row.is_active?'启用':'禁用'}}</el-tag></template></el-table-column>
        <el-table-column label="操作" width="160"><template #d="r">
          <el-button link type="primary" size="small" @click="edit(r.row)">编辑</el-button>
          <el-button link type="danger" size="small" @click="del(r.row.id)">删除</el-button>
        </template></el-table-column>
      </el-table>
    </div>
    <el-dialog v-model="dlg" :title="editing?'编辑':'新建'" width="500px">
      <el-form :model="f"><el-form-item label="名称"><el-input v-model="f.name"/></el-form-item><el-form-item label="域名"><el-input v-model="f.domain"/></el-form-item><el-form-item label="模板"><el-input v-model="f.template"/></el-form-item><el-form-item label="语言"><el-input v-model="f.language"/></el-form-item></el-form>
      <template #footer><el-button @click="dlg=false">取消</el-button><el-button type="primary" @click="save">{{editing?'保存':'创建'}}</el-button></template>
    </el-dialog>
  </div>`,
  setup() {
    const items=ref([]); const ld=ref(false); const dlg=ref(false); const editing=ref(false);
    const f=reactive({id:'',name:'',domain:'',template:'default',language:'zh'});
    const load=async()=>{ ld.value=true; try { const r=await API.get('/sites'); items.value=r.data; }catch(e){} ld.value=false; };
    const add=()=>{ editing.value=false; f.id='';f.name='';f.domain='';f.template='default';f.language='zh'; dlg.value=true; };
    const edit=(s)=>{ editing.value=true; Object.assign(f,s); dlg.value=true; };
    const save=async()=>{
      try {
        const d={name:f.name,domain:f.domain,template:f.template,language:f.language};
        if(editing.value) await API.put('/sites/'+f.id,d); else await API.post('/sites',d);
        ElMessage.success(editing.value?'已保存':'创建成功'); dlg.value=false; load();
      }catch(e){ElMessage.error('操作失败');}
    };
    const del=async(id)=>{ try { await ElMessageBox.confirm('确定删除？','警告',{type:'warning'}); await API.delete('/sites/'+id); ElMessage.success('已删除'); load(); }catch(e){} };
    onMounted(load);
    return { items,ld,dlg,editing,f,add,edit,save,del };
  }
};

// ── Link Rings ───────────────────────────────────────────────────────────
const LinkRings = {
  template: `
  <div>
    <div class="page-header"><h2>🔗 链轮管理</h2></div>
    <div class="card">
      <div class="toolbar"><div></div><el-button type="primary" @click="add">+ 新建</el-button></div>
      <el-table :data="items" stripe v-loading="ld">
        <el-table-column prop="name" label="名称" width="150"/><el-table-column prop="ring_type" label="类型" width="120"/>
        <el-table-column prop="max_links" label="最大" width="70"/><el-table-column prop="display_mode" label="模式" width="90"/>
        <el-table-column label="目标" width="70"><template #d="r">{{(r.row.targets||[]).length}}</template></el-table-column>
        <el-table-column label="操作" width="100"><template #d="r">
          <el-button link type="danger" size="small" @click="del(r.row.id)">删除</el-button>
        </template></el-table-column>
      </el-table>
    </div>
    <el-dialog v-model="dlg" title="新建链轮" width="500px">
      <el-form :model="f"><el-form-item label="名称"><el-input v-model="f.name"/></el-form-item><el-form-item label="类型"><el-select v-model="f.ring_type"><el-option label="跨站" value="cross_site"/><el-option label="站内书" value="same_site_books"/><el-option label="自定义" value="custom"/></el-select></el-form-item><el-form-item label="最大链接"><el-input-number v-model="f.max_links"/></el-form-item><el-form-item label="模式"><el-select v-model="f.display_mode"><el-option label="侧边栏" value="sidebar"/><el-option label="底部" value="footer"/><el-option label="内联" value="inline"/></el-select></el-form-item></el-form>
      <template #footer><el-button @click="dlg=false">取消</el-button><el-button type="primary" @click="save">创建</el-button></template>
    </el-dialog>
  </div>`,
  setup() {
    const items=ref([]); const ld=ref(false); const dlg=ref(false);
    const f=reactive({name:'',ring_type:'cross_site',max_links:10,display_mode:'sidebar'});
    const load=async()=>{ ld.value=true; try { const r=await API.get('/link-rings'); items.value=r.data; }catch(e){} ld.value=false; };
    const add=()=>{ f.name='';f.ring_type='cross_site';f.max_links=10;f.display_mode='sidebar'; dlg.value=true; };
    const save=async()=>{ try { await API.post('/link-rings',f); ElMessage.success('创建成功'); dlg.value=false; load(); }catch(e){ElMessage.error('创建失败');} };
    const del=async(id)=>{ try { await ElMessageBox.confirm('确定删除？','警告',{type:'warning'}); await API.delete('/link-rings/'+id); ElMessage.success('已删除'); load(); }catch(e){} };
    onMounted(load);
    return { items,ld,dlg,f,add,save,del };
  }
};

// ── Crawler ──────────────────────────────────────────────────────────────
const Crawler = {
  template: `
  <div>
    <div class="page-header"><h2>🕷️ 采集任务</h2></div>
    <div class="card">
      <div class="toolbar">
        <el-select v-model="st" placeholder="状态" clearable style="width:110px" @change="load"><el-option label="待处理" value="pending"/><el-option label="运行中" value="running"/><el-option label="已完成" value="completed"/><el-option label="失败" value="failed"/></el-select>
        <el-button @click="load">刷新</el-button>
        <el-button type="primary" @click="trigger">+ 触发采集</el-button>
      </div>
      <el-table :data="items" stripe v-loading="ld">
        <el-table-column label="小说ID" width="120"><template #d="r">{{(r.row.novel_id||'').substring(0,8)}}...</template></el-table-column>
        <el-table-column label="状态" width="80"><template #d="r"><el-tag :type="tp(r.row.status)" size="small">{{r.row.status}}</el-tag></template></el-table-column>
        <el-table-column prop="chapters_found" label="发现" width="60"/><el-table-column prop="chapters_added" label="新增" width="60"/>
        <el-table-column label="错误" min-width="150"><template #d="r"><span style="color:#f56c6c;font-size:12px">{{r.row.error_message?.String||''}}</span></template></el-table-column>
        <el-table-column label="操作" width="200"><template #d="r">
          <el-button link size="small" @click="act(r.row.id,'start')" v-if="r.row.status==='pending'">启动</el-button>
          <el-button link size="small" type="danger" @click="act(r.row.id,'stop')" v-if="r.row.status==='running'">停止</el-button>
          <el-button link size="small" @click="act(r.row.id,'retry')" v-if="r.row.status==='failed'">重试</el-button>
          <el-button link size="small" type="danger" @click="delT(r.row.id)">删除</el-button>
        </template></el-table-column>
      </el-table>
      <div style="margin-top:14px;text-align:right"><el-pagination background layout="total,prev,next" :total="total" :page-size="20" v-model:current-page="pg" @current-change="load"/></div>
    </div>
    <el-dialog v-model="tdlg" title="触发采集" width="400px">
      <el-form :model="tf"><el-form-item label="小说ID"><el-input v-model="tf.novel_id"/></el-form-item><el-form-item label="源名称"><el-input v-model="tf.source_name"/></el-form-item></el-form>
      <template #footer><el-button @click="tdlg=false">取消</el-button><el-button type="primary" @click="doTrigger">触发</el-button></template>
    </el-dialog>
  </div>`,
  setup() {
    const items=ref([]); const total=ref(0); const pg=ref(1); const ld=ref(false); const st=ref(''); const tdlg=ref(false);
    const tf=reactive({novel_id:'',source_name:'23qb'});
    const tp=s=>({pending:'info',running:'warning',completed:'success',failed:'danger'}[s]||'info');
    const load=async()=>{ ld.value=true; try { const r=await API.get('/crawler/tasks',{params:{page:pg.value,status:st.value}}); items.value=r.data.items; total.value=r.data.total; }catch(e){} ld.value=false; };
    const trigger=()=>{ tf.novel_id='';tf.source_name='23qb'; tdlg.value=true; };
    const doTrigger=async()=>{ try { await API.post('/crawler/trigger',tf); ElMessage.success('任务已创建'); tdlg.value=false; load(); }catch(e){ElMessage.error(e.response?.data?.error||'触发失败');} };
    const act=async(id,a)=>{ try { await API.post('/crawler/tasks/'+id+'/'+a); ElMessage.success('操作成功'); load(); }catch(e){ElMessage.error('操作失败');} };
    const delT=async(id)=>{ try { await ElMessageBox.confirm('确定删除？','警告',{type:'warning'}); await API.delete('/crawler/tasks/'+id); ElMessage.success('已删除'); load(); }catch(e){} };
    onMounted(load);
    return { items,total,pg,ld,st,tdlg,tf,tp,load,trigger,doTrigger,act,delT };
  }
};

// ── Cache / Repair ───────────────────────────────────────────────────────
const Cache = {
  template: `
  <div>
    <div class="page-header"><h2>🗄️ 缓存运维</h2></div>
    <div class="stat-grid">
      <div class="stat-card"><div class="icon blue"><el-icon size="22"><Monitor/></el-icon></div><div><div class="value">{{mem.alloc_mb||0}} MB</div><div class="label">内存占用</div></div></div>
      <div class="stat-card"><div class="icon green"><el-icon size="22"><Connection/></el-icon></div><div><div class="value">{{mem.goroutines||0}}</div><div class="label">Goroutines</div></div></div>
      <div class="stat-card"><div class="icon orange"><el-icon size="22"><RefreshRight/></el-icon></div><div><div class="value">{{mem.num_gc||0}}</div><div class="label">GC次数</div></div></div>
    </div>
    <div class="card">
      <h3 style="margin-bottom:14px">缓存</h3>
      <el-button type="primary" @click="flush">刷新缓存</el-button>
    </div>
    <div class="card">
      <h3 style="margin-bottom:14px">数据修复</h3>
      <el-descriptions :column="2" border v-if="rs">
        <el-descriptions-item label="空章节">{{rs.empty_chapters}}</el-descriptions-item>
        <el-descriptions-item label="无封面">{{rs.no_cover}}</el-descriptions-item>
        <el-descriptions-item label="无简介">{{rs.no_description}}</el-descriptions-item>
        <el-descriptions-item label="无作者">{{rs.no_author}}</el-descriptions-item>
      </el-descriptions>
      <div style="margin-top:12px"><el-button type="warning" @click="repair">修复空章节</el-button></div>
    </div>
  </div>`,
  setup() {
    const mem=ref({}); const rs=ref(null);
    onMounted(async()=>{
      API.get('/cache/health').then(r=>mem.value=r.data.memory||{}).catch(()=>{});
      API.get('/repair/status').then(r=>rs.value=r.data).catch(()=>{});
    });
    const flush=async()=>{ try { await API.post('/cache/flush'); ElMessage.success('已刷新'); }catch(e){} };
    const repair=async()=>{ try { await API.post('/repair/chapters'); ElMessage.success('已启动'); }catch(e){ElMessage.warning('Go版本开发中');} };
    return { mem,rs,flush,repair };
  }
};

// ── Router ───────────────────────────────────────────────────────────────
const routes = [
  { path: '/', redirect: '/dashboard' },
  { path: '/dashboard', component: Dashboard, meta: { title: '控制台' } },
  { path: '/novels', component: Novels, meta: { title: '小说管理' } },
  { path: '/novels/:id', component: NovelDetail, props: true, meta: { title: '小说详情' } },
  { path: '/categories', component: Categories, meta: { title: '分类管理' } },
  { path: '/sites', component: Sites, meta: { title: '站点管理' } },
  { path: '/link-rings', component: LinkRings, meta: { title: '链轮管理' } },
  { path: '/crawler', component: Crawler, meta: { title: '采集任务' } },
  { path: '/cache', component: Cache, meta: { title: '缓存运维' } },
];
const router = createRouter({ history: createWebHashHistory(), routes });

// ── Root App ─────────────────────────────────────────────────────────────
const AppComponent = {
  template: '<login-page v-if="!loggedIn"/><main-layout v-else/>',
  components: { LoginPage, MainLayout },
  setup() { const loggedIn = computed(() => !!atok()); return { loggedIn }; }
};

const app = createApp(AppComponent);
app.use(router);
app.use(ElementPlus, { locale: ElementPlus.localeZhCn || {} });
for (const [k, v] of Object.entries(Icons)) app.component(k, v);
app.mount('#app');
