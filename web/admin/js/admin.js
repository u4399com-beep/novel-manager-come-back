/* 归来小说CMS - Admin Panel v3.0 — Full Feature Replication */
const { createApp, ref, reactive, computed, onMounted, onUnmounted, watch, nextTick } = Vue;
const { createRouter, createWebHashHistory, useRoute, useRouter } = VueRouter;
const { ElMessage, ElMessageBox, ElNotification } = ElementPlus;
const Icons = window.ElementPlusIconsVue || {};

const API = axios.create({ baseURL: '/api/v1' });
API.interceptors.request.use(c => {
  const t = localStorage.getItem('atok');
  if (t) c.headers.Authorization = 'Bearer ' + t;
  return c;
});
API.interceptors.response.use(r => r, e => {
  if (e.response?.status === 401) { localStorage.removeItem('atok'); location.hash = '#/login'; }
  return Promise.reject(e);
});

const atok = () => localStorage.getItem('atok') || '';
const setAtok = (t, u) => { localStorage.setItem('atok', t); localStorage.setItem('auser', u); };
const logout = () => { localStorage.removeItem('atok'); localStorage.removeItem('auser'); location.hash = '#/login'; };

// ── Login ─────────────────────────────────────────────────────────────────
const LoginPage = {
  template: `
  <div class="login-container">
    <div class="login-card">
      <h1>📚 归来小说CMS</h1><p class="sub">管理后台</p>
      <el-tabs v-model="tab">
        <el-tab-pane label="登录" name="login">
          <el-form @submit.prevent="login">
            <el-form-item><el-input v-model="lf.username" placeholder="用户名" size="large" @keyup.enter="login"/></el-form-item>
            <el-form-item><el-input v-model="lf.password" type="password" placeholder="密码" show-password size="large" @keyup.enter="login"/></el-form-item>
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
      try { const r = await axios.post('/api/v1/login', lf); setAtok(r.data.access_token, lf.username); location.hash = '#/dashboard'; location.reload(); }
      catch (e) { ElMessage.error(e.response?.data?.error || '登录失败'); }
      loading.value = false;
    };
    const register = async () => {
      if (rf.password.length < 8) return ElMessage.warning('密码至少8位');
      rloading.value = true;
      try { await axios.post('/api/v1/register', rf); ElMessage.success('注册成功'); tab.value = 'login'; lf.username = rf.username; }
      catch (e) { ElMessage.error(e.response?.data?.error || '注册失败'); }
      rloading.value = false;
    };
    return { tab, loading, rloading, lf, rf, login, register };
  }
};

// ── Layout ────────────────────────────────────────────────────────────────
const MainLayout = {
  template: `
  <div style="display:flex;width:100%">
    <div class="sidebar" :class="{collapsed:collapsed}">
      <a class="logo" @click="collapsed=!collapsed">📚<span v-show="!collapsed"> 归来CMS</span></a>
      <a v-for="m in menu" :key="m.path" :href="'#'+m.path" class="menu-item" :class="{active:route.path===m.path}" @click="navigate(m.path)">
        <span class="menu-icon">{{m.icon}}</span>
        <span class="menu-label">{{m.label}}</span>
      </a>
    </div>
    <div class="main-area" :class="{expanded:collapsed}">
      <div class="header">
        <span class="title">{{pageTitle}}</span>
        <div class="user-info"><span>{{user}}</span><el-button link type="danger" @click="logout">退出</el-button></div>
      </div>
      <div class="content"><router-view/></div>
    </div>
  </div>`,
  setup() {
    const route = useRoute(); const router = useRouter(); const collapsed = ref(false);
    const pageTitle = computed(() => route.meta.title || '');
    const user = localStorage.getItem('auser') || '';
    const menu = [
      { path: '/dashboard', label: '控制台', icon: '📊' },
      { path: '/novels', label: '小说管理', icon: '📚' },
      { path: '/categories', label: '分类管理', icon: '🏷️' },
      { path: '/crawler', label: '采集任务', icon: '🕷️' },
      { path: '/sites', label: '站点管理', icon: '🌐' },
      { path: '/link-rings', label: '链轮管理', icon: '🔗' },
      { path: '/cache', label: '缓存运维', icon: '🗄️' },
    ];
    const navigate = (path) => router.push(path);
    return { route, pageTitle, user, collapsed, menu, navigate, logout };
  }
};

// ── Dashboard ─────────────────────────────────────────────────────────────
const Dashboard = {
  template: `
  <div>
    <div class="page-header"><h2>📊 控制台</h2></div>
    <div class="stat-grid">
      <div class="stat-card" v-for="s in stats" :key="s.l"><div class="icon" :class="s.c"><el-icon :size="22"><component :is="s.i"/></el-icon></div><div><div class="value">{{s.v}}</div><div class="label">{{s.l}}</div></div></div>
    </div>
    <div class="card" style="margin-bottom:16px">
      <h3 style="margin-bottom:12px">系统状态</h3>
      <el-descriptions :column="2" border>
        <el-descriptions-item label="服务状态">{{health.status||'-'}}</el-descriptions-item>
        <el-descriptions-item label="版本">{{health.version||'-'}}</el-descriptions-item>
        <el-descriptions-item label="数据库">{{health.database||'-'}}</el-descriptions-item>
        <el-descriptions-item label="内存">{{mem.alloc_mb||0}} MB</el-descriptions-item>
        <el-descriptions-item label="Goroutines">{{mem.goroutines||0}}</el-descriptions-item>
        <el-descriptions-item label="GC">{{mem.num_gc||0}}</el-descriptions-item>
      </el-descriptions>
    </div>
    <div class="card">
      <h3 style="margin-bottom:12px">数据修复工具</h3>
      <el-descriptions :column="2" border v-if="repair">
        <el-descriptions-item label="空章节">{{repair.empty_chapters||0}}</el-descriptions-item>
        <el-descriptions-item label="无封面">{{repair.no_cover||0}}</el-descriptions-item>
        <el-descriptions-item label="无简介">{{repair.no_description||0}}</el-descriptions-item>
        <el-descriptions-item label="无作者">{{repair.no_author||0}}</el-descriptions-item>
      </el-descriptions>
      <div style="margin-top:12px"><el-button @click="loadRepair">刷新状态</el-button><el-button type="warning" @click="repairCh">修复空章节</el-button></div>
    </div>
  </div>`,
  setup() {
    const stats = ref([]); const health = ref({}); const mem = ref({}); const repair = ref({});
    const loadAll = async () => {
      axios.get('/health').then(r => health.value = r.data).catch(() => {});
      API.get('/crawler/stats').then(r => { const d = r.data; stats.value = [
        { l: '小说总数', v: d.novels || 0, i: 'Reading', c: 'blue' },
        { l: '章节总数', v: d.chapters || 0, i: 'Collection', c: 'green' },
        { l: '采集任务', v: d.tasks_total || 0, i: 'Download', c: 'orange' },
        { l: '待处理', v: d.tasks_pending || 0, i: 'Clock', c: 'red' },
      ]; }).catch(() => {});
      API.get('/cache/health').then(r => { mem.value = r.data.memory || {}; }).catch(() => {});
      API.get('/repair/status').then(r => { repair.value = r.data; }).catch(() => {});
    };
    const repairCh = async () => { try { await API.post('/repair/chapters'); ElMessage.success('已启动'); } catch (e) { ElMessage.warning('Go版开发中'); } };
    onMounted(loadAll);
    return { stats, health, mem, repair, loadRepair: loadAll, repairCh };
  }
};

// ── NovelList ─────────────────────────────────────────────────────────────
const NovelList = {
  template: `
  <div>
    <div class="page-header"><h2>📚 小说管理</h2></div>
    <div class="card">
      <div class="toolbar">
        <div class="toolbar-left">
          <el-input v-model="search" placeholder="搜索标题/作者..." clearable style="width:180px" @clear="load(1)" @keyup.enter="load(1)"/>
          <el-select v-model="catFilter" placeholder="分类" clearable style="width:110px" @change="load(1)"><el-option v-for="c in categories" :key="c.id" :label="c.name" :value="c.id"/></el-select>
          <el-select v-model="statusFilter" placeholder="状态" clearable style="width:100px" @change="load(1)"><el-option label="连载中" value="ongoing"/><el-option label="已完结" value="completed"/><el-option label="暂停" value="hiatus"/></el-select>
          <el-button @click="load(1)">搜索</el-button>
        </div>
        <el-button type="primary" @click="$router.push('/novels/create')">+ 新建小说</el-button>
      </div>
      <el-table :data="items" stripe v-loading="loading" @row-click="(r)=>$router.push('/novels/'+r.id)" style="cursor:pointer" @sort-change="onSort">
        <el-table-column prop="title" label="书名" min-width="180" sortable="custom"/>
        <el-table-column prop="author" label="作者" width="120" sortable="custom"/>
        <el-table-column label="状态" width="80"><template #d="s"><el-tag :type="s.row.status==='ongoing'?'success':s.row.status==='completed'?'info':'warning'" size="small">{{statusMap[s.row.status]||s.row.status}}</el-tag></template></el-table-column>
        <el-table-column prop="total_chapters" label="章节" width="80" sortable="custom"/>
        <el-table-column label="分类" width="150"><template #d="s"><span v-if="(s.row.categories||[]).length">{{s.row.categories.map(c=>c.name).join(', ')}}</span><span v-else>-</span></template></el-table-column>
        <el-table-column label="操作" width="180" fixed="right"><template #d="s">
          <el-button link type="primary" size="small" @click.stop="$router.push('/novels/'+s.row.id)">详情</el-button>
          <el-button link size="small" @click.stop="$router.push('/novels/'+s.row.id+'/edit')">编辑</el-button>
          <el-popconfirm title="确定删除该小说及所有章节?" @confirm="del(s.row.id)"><template #reference><el-button link type="danger" size="small">删除</el-button></template></el-popconfirm>
        </template></el-table-column>
      </el-table>
      <div style="margin-top:14px;text-align:right"><el-pagination background layout="total,sizes,prev,pager,next" :total="total" v-model:page-size="size" :page-sizes="[10,20,50]" v-model:current-page="page" @size-change="load(1)" @current-change="load"/></div>
    </div>
  </div>`,
  setup() {
    const items = ref([]); const total = ref(0); const page = ref(1); const size = ref(20); const loading = ref(false);
    const search = ref(''); const catFilter = ref(''); const statusFilter = ref(''); const categories = ref([]);
    const sortBy = ref('updated_at'); const sortDir = ref('desc');
    const statusMap = { ongoing: '连载中', completed: '已完结', hiatus: '暂停' };
    const load = async (p) => {
      if (p) page.value = p;
      loading.value = true;
      try {
        const r = await API.get('/novels', { params: { page: page.value, size: size.value, search: search.value, category_id: catFilter.value || undefined, status: statusFilter.value || undefined, sort_by: sortBy.value, sort_dir: sortDir.value } });
        items.value = r.data.items; total.value = r.data.total;
      } catch (e) {}
      loading.value = false;
    };
    const onSort = ({ prop, order }) => { sortBy.value = prop || 'updated_at'; sortDir.value = order === 'ascending' ? 'asc' : 'desc'; load(1); };
    const del = async (id) => { try { await API.delete('/novels/' + id); ElMessage.success('已删除'); load(); } catch (e) { ElMessage.error('删除失败'); } };
    const loadCats = async () => { try { const r = await API.get('/categories'); categories.value = r.data; } catch (e) {} };
    onMounted(() => { load(); loadCats(); });
    return { items, total, page, size, loading, search, catFilter, statusFilter, categories, statusMap, load, onSort, del };
  }
};

// ── NovelForm (Create/Edit) ──────────────────────────────────────────────
const NovelForm = {
  props: ['id'],
  template: `
  <div>
    <div class="page-header"><h2>{{isEdit?'编辑小说':'新建小说'}}</h2></div>
    <div class="card" style="max-width:700px">
      <el-form :model="f" label-width="90px" @submit.prevent="save">
        <el-form-item label="书名" required><el-input v-model="f.title"/></el-form-item>
        <el-form-item label="作者"><el-input v-model="f.author"/></el-form-item>
        <el-form-item label="状态"><el-radio-group v-model="f.status"><el-radio-button value="ongoing">连载中</el-radio-button><el-radio-button value="completed">已完结</el-radio-button><el-radio-button value="hiatus">暂停</el-radio-button></el-radio-group></el-form-item>
        <el-form-item label="分类"><el-select v-model="f.category_ids" multiple placeholder="选择分类"><el-option v-for="c in categories" :key="c.id" :label="c.name" :value="c.id"/></el-select></el-form-item>
        <el-form-item label="简介"><el-input v-model="f.description" type="textarea" :rows="4"/></el-form-item>
        <el-form-item label="源名称"><el-input v-model="f.source_name" placeholder="如: 23qb"/></el-form-item>
        <el-form-item label="源URL"><el-input v-model="f.source_url" placeholder="外部小说源地址"/></el-form-item>
        <el-form-item v-if="!isEdit" label="封面"><input type="file" accept="image/*" @change="onCover" ref="coverInput"/><img v-if="coverPreview" :src="coverPreview" style="width:120px;height:160px;object-fit:cover;margin-top:8px;border-radius:6px"/></el-form-item>
        <el-form-item v-if="isEdit && f.cover_image_url" label="封面"><img :src="f.cover_image_url" style="width:120px;height:160px;object-fit:cover;border-radius:6px"/></el-form-item>
        <el-form-item><el-button type="primary" @click="save" :loading="saving">{{isEdit?'保存':'创建'}}</el-button><el-button @click="$router.back()">取消</el-button></el-form-item>
      </el-form>
    </div>
  </div>`,
  setup(props) {
    const isEdit = computed(() => !!props.id && props.id !== 'create');
    const f = reactive({ title: '', author: '', status: 'ongoing', category_ids: [], description: '', source_name: '', source_url: '', cover_image_url: '' });
    const categories = ref([]); const saving = ref(false); const coverPreview = ref(''); const coverFile = ref(null);
    const loadCats = async () => { try { const r = await API.get('/categories'); categories.value = r.data; } catch (e) {} };
    const loadNovel = async () => { try { const r = await API.get('/novels/' + props.id); Object.assign(f, r.data); f.category_ids = (r.data.categories || []).map(c => c.id); } catch (e) { ElMessage.error('小说不存在'); $router.push('/novels'); } };
    const onCover = (e) => { const file = e.target.files[0]; if (file) { coverFile.value = file; coverPreview.value = URL.createObjectURL(file); } };
    const save = async () => {
      if (!f.title) return ElMessage.warning('请输入书名');
      saving.value = true;
      try {
        const payload = { title: f.title, author: f.author, status: f.status, category_ids: f.category_ids, description: f.description, source_name: f.source_name, source_url: f.source_url };
        if (isEdit.value) {
          await API.put('/novels/' + props.id, payload);
          ElMessage.success('已保存');
        } else {
          const r = await API.post('/novels', payload);
          if (coverFile.value) { const fd = new FormData(); fd.append('file', coverFile.value); await API.post('/novels/' + r.data.id + '/cover', fd, { headers: { 'Content-Type': 'multipart/form-data' } }); }
          ElMessage.success('创建成功'); $router.push('/novels');
        }
      } catch (e) { ElMessage.error('操作失败'); }
      saving.value = false;
    };
    onMounted(() => { loadCats(); if (isEdit.value) loadNovel(); });
    return { isEdit, f, categories, saving, coverPreview, onCover, save };
  }
};

// ── NovelDetail ───────────────────────────────────────────────────────────
const NovelDetailPage = {
  props: ['id'],
  template: `
  <div>
    <div class="page-header"><h2>📖 {{novel.title||'加载中...'}}</h2><p><el-button link @click="$router.push('/novels')">← 返回列表</el-button></p></div>
    <div class="card" v-if="novel.id">
      <el-descriptions :column="2" border>
        <el-descriptions-item label="书名">{{novel.title}}</el-descriptions-item>
        <el-descriptions-item label="作者">{{novel.author||'-'}}</el-descriptions-item>
        <el-descriptions-item label="状态"><el-tag :type="novel.status==='ongoing'?'success':novel.status==='completed'?'info':'warning'" size="small">{{statusMap[novel.status]}}</el-tag></el-descriptions-item>
        <el-descriptions-item label="章节数">{{novel.total_chapters}}</el-descriptions-item>
        <el-descriptions-item label="分类">{{(novel.categories||[]).map(c=>c.name).join(', ')||'-'}}</el-descriptions-item>
        <el-descriptions-item label="来源">{{novel.source_name||'-'}}</el-descriptions-item>
        <el-descriptions-item label="简介" :span="2">{{novel.description||'暂无'}}</el-descriptions-item>
      </el-descriptions>
      <div style="margin-top:14px">
        <el-button type="primary" @click="$router.push('/novels/'+novel.id+'/edit')">编辑</el-button>
        <el-button type="success" @click="$router.push('/novels/'+novel.id+'/chapters')">管理章节</el-button>
      </div>
    </div>
    <div class="card" style="margin-top:16px" v-if="novel.id">
      <h3 style="margin-bottom:12px">统计信息</h3>
      <el-descriptions :column="3" border>
        <el-descriptions-item label="总章节">{{stats.total_chapters||0}}</el-descriptions-item>
        <el-descriptions-item label="已发布">{{stats.published_chapters||0}}</el-descriptions-item>
        <el-descriptions-item label="总字数">{{(stats.total_words||0).toLocaleString()}}</el-descriptions-item>
      </el-descriptions>
    </div>
    <div class="card" style="margin-top:16px">
      <h3 style="margin-bottom:12px">最新章节</h3>
      <el-table :data="chapters" stripe>
        <el-table-column prop="sort_order" label="#" width="60"/>
        <el-table-column prop="title" label="标题" min-width="200"><template #d="s"><a :href="'#/novels/'+novel.id+'/chapters/'+s.row.id" style="color:var(--primary)">{{s.row.title}}</a></template></el-table-column>
        <el-table-column prop="word_count" label="字数" width="80"/>
        <el-table-column label="状态" width="80"><template #d="s"><el-tag :type="s.row.is_published?'success':'info'" size="small">{{s.row.is_published?'已发布':'草稿'}}</el-tag></template></el-table-column>
      </el-table>
      <div style="margin-top:12px"><el-button @click="$router.push('/novels/'+novel.id+'/chapters')">查看全部章节 →</el-button></div>
    </div>
  </div>`,
  setup(props) {
    const novel = ref({}); const stats = ref({}); const chapters = ref([]);
    const statusMap = { ongoing: '连载中', completed: '已完结', hiatus: '暂停' };
    onMounted(async () => {
      try { const r = await API.get('/novels/' + props.id); novel.value = r.data; } catch (e) { ElMessage.error('不存在'); $router.push('/novels'); }
      try { const r = await API.get('/novels/' + props.id + '/statistics'); stats.value = r.data; } catch (e) {}
      try { const r = await API.get('/novels/' + props.id + '/chapters', { params: { size: 10 } }); chapters.value = r.data.items; } catch (e) {}
    });
    return { novel, stats, chapters, statusMap };
  }
};

// ── ChapterList ───────────────────────────────────────────────────────────
const ChapterList = {
  props: ['novelId'],
  template: `
  <div>
    <div class="page-header"><h2>📖 章节管理</h2><p><el-button link @click="$router.push('/novels/'+novelId)">← 返回小说</el-button></p></div>
    <div class="card">
      <div class="toolbar">
        <div><el-button type="primary" @click="addOne">+ 添加章节</el-button><el-button @click="showBatch=true">批量添加</el-button><el-popconfirm title="确定删除选中章节?" @confirm="batchDel"><template #reference><el-button type="danger" :disabled="!selected.length">批量删除({{selected.length}})</el-button></template></el-popconfirm></div>
      </div>
      <el-table :data="items" stripe v-loading="loading" @selection-change="s=>selected=s.map(r=>r.id)" ref="tbl">
        <el-table-column type="selection" width="40"/>
        <el-table-column prop="sort_order" label="#" width="60"/>
        <el-table-column prop="title" label="标题" min-width="220"><template #d="s"><a :href="'#/novels/'+novelId+'/chapters/'+s.row.id" style="color:var(--primary)">{{s.row.title}}</a></template></el-table-column>
        <el-table-column prop="word_count" label="字数" width="80"/>
        <el-table-column label="状态" width="80"><template #d="s"><el-tag :type="s.row.is_published?'success':'info'" size="small">{{s.row.is_published?'已发布':'草稿'}}</el-tag></template></el-table-column>
        <el-table-column label="操作" width="120" fixed="right"><template #d="s">
          <el-button link type="primary" size="small" @click="$router.push('/novels/'+novelId+'/chapters/'+s.row.id)">编辑</el-button>
          <el-popconfirm title="确定删除?" @confirm="del(s.row.id)"><template #reference><el-button link type="danger" size="small">删除</el-button></template></el-popconfirm>
        </template></el-table-column>
      </el-table>
      <div style="margin-top:14px;text-align:right"><el-pagination background layout="total,prev,pager,next" :total="total" :page-size="50" v-model:current-page="page" @current-change="load"/></div>
    </div>
    <el-dialog v-model="showBatch" title="批量添加章节" width="500px">
      <p style="color:#909399;margin-bottom:12px;font-size:13px">每行一个章节标题，按顺序自动编号</p>
      <el-input v-model="batchTitles" type="textarea" :rows="10" placeholder="第一章&#10;第二章&#10;第三章"/>
      <template #footer><el-button @click="showBatch=false">取消</el-button><el-button type="primary" @click="batchCreate" :loading="batching">批量创建</el-button></template>
    </el-dialog>
  </div>`,
  setup(props) {
    const items = ref([]); const total = ref(0); const page = ref(1); const loading = ref(false); const selected = ref([]);
    const showBatch = ref(false); const batchTitles = ref(''); const batching = ref(false);
    const load = async () => { loading.value = true; try { const r = await API.get('/novels/' + props.novelId + '/chapters', { params: { page: page.value, size: 50 } }); items.value = r.data.items; total.value = r.data.total; } catch (e) {} loading.value = false; };
    const addOne = () => $router.push('/novels/' + props.novelId + '/chapters/new');
    const del = async (id) => { try { await API.delete('/novels/' + props.novelId + '/chapters/' + id); ElMessage.success('已删除'); load(); } catch (e) { ElMessage.error('删除失败'); } };
    const batchDel = async () => { try { await API.post('/novels/' + props.novelId + '/chapters/batch', { ids: selected.value }, { params: { _method: 'DELETE' } }); ElMessage.success('已删除'); load(); } catch (e) { try { await axios({ method: 'DELETE', url: '/api/v1/novels/' + props.novelId + '/chapters/batch', data: { ids: selected.value }, headers: { Authorization: 'Bearer ' + atok() } }); ElMessage.success('已删除'); load(); } catch (e2) { ElMessage.error('删除失败'); } } };
    const batchCreate = async () => {
      const titles = batchTitles.value.split('\n').map(s => s.trim()).filter(Boolean);
      if (!titles.length) return ElMessage.warning('请输入章节标题');
      batching.value = true;
      try { await API.post('/novels/' + props.novelId + '/chapters/batch', { chapters: titles.map(t => ({ title: t })) }); ElMessage.success(`创建 ${titles.length} 章`); showBatch.value = false; batchTitles.value = ''; load(); } catch (e) { ElMessage.error('创建失败'); }
      batching.value = false;
    };
    onMounted(load);
    return { items, total, page, loading, selected, showBatch, batchTitles, batching, load, addOne, del, batchDel, batchCreate, $router };
  }
};

// ── ChapterEditor ─────────────────────────────────────────────────────────
const ChapterEditor = {
  props: ['novelId', 'chapterId'],
  template: `
  <div>
    <div class="page-header"><h2>{{isNew?'新建章节':'编辑章节'}}</h2><p><el-button link @click="$router.push('/novels/'+novelId+'/chapters')">← 返回列表</el-button></p></div>
    <div class="card" style="max-width:800px">
      <el-form :model="f" label-width="80px">
        <el-form-item label="标题" required><el-input v-model="f.title"/></el-form-item>
        <el-form-item label="排序"><el-input-number v-model="f.sort_order" :min="1"/></el-form-item>
        <el-form-item label="发布"><el-switch v-model="f.is_published" active-text="已发布" inactive-text="草稿"/></el-form-item>
        <el-form-item label="内容"><el-input v-model="f.content" type="textarea" :rows="18"/></el-form-item>
        <el-form-item><span style="color:#909399;font-size:13px">字数: {{wordCount}}</span></el-form-item>
        <el-form-item><el-button type="primary" @click="save" :loading="saving">保存</el-button><el-button @click="$router.back()">取消</el-button></el-form-item>
      </el-form>
    </div>
  </div>`,
  setup(props) {
    const isNew = computed(() => props.chapterId === 'new');
    const f = reactive({ title: '', sort_order: undefined, is_published: true, content: '' });
    const saving = ref(false);
    const wordCount = computed(() => { const t = f.content || ''; const cn = (t.match(/[一-鿿]/g) || []).length; const en = (t.match(/[a-zA-Z]+/g) || []).length; return cn + en; });
    const loadCh = async () => { try { const r = await API.get('/novels/' + props.novelId + '/chapters/' + props.chapterId); Object.assign(f, r.data); } catch (e) { ElMessage.error('章节不存在'); $router.back(); } };
    const save = async () => {
      if (!f.title) return ElMessage.warning('请输入标题');
      saving.value = true;
      try {
        const data = { title: f.title, content: f.content, sort_order: f.sort_order, is_published: f.is_published };
        if (isNew.value) { const r = await API.post('/novels/' + props.novelId + '/chapters', data); ElMessage.success('创建成功'); $router.push('/novels/' + props.novelId + '/chapters/' + r.data.id); }
        else { await API.put('/novels/' + props.novelId + '/chapters/' + props.chapterId, data); ElMessage.success('已保存'); }
      } catch (e) { ElMessage.error('操作失败'); }
      saving.value = false;
    };
    onMounted(() => { if (!isNew.value) loadCh(); });
    return { isNew, f, saving, wordCount, save };
  }
};

// ── Categories ────────────────────────────────────────────────────────────
const CategoriesC = {
  template: `
  <div>
    <div class="page-header"><h2>🏷️ 分类管理</h2></div>
    <div class="card">
      <div class="toolbar"><div></div><el-button type="primary" @click="open()">+ 新建分类</el-button></div>
      <el-table :data="items" stripe v-loading="loading">
        <el-table-column prop="id" label="ID" width="60"/><el-table-column prop="name" label="名称" width="150"/>
        <el-table-column prop="slug" label="标识" width="150"/><el-table-column prop="sort_order" label="排序" width="80"/>
        <el-table-column label="操作" width="160"><template #d="r">
          <el-button link type="primary" size="small" @click="open(r.row)">编辑</el-button>
          <el-popconfirm title="确定删除?" @confirm="del(r.row.id)"><template #reference><el-button link type="danger" size="small">删除</el-button></template></el-popconfirm>
        </template></el-table-column>
      </el-table>
    </div>
    <el-dialog v-model="dlg" :title="editing?'编辑分类':'新建分类'" width="420px">
      <el-form :model="f"><el-form-item label="名称"><el-input v-model="f.name"/></el-form-item><el-form-item label="标识"><el-input v-model="f.slug"/></el-form-item><el-form-item label="排序"><el-input-number v-model="f.sort_order" :min="0"/></el-form-item></el-form>
      <template #footer><el-button @click="dlg=false">取消</el-button><el-button type="primary" @click="save" :loading="saving">{{editing?'保存':'创建'}}</el-button></template>
    </el-dialog>
  </div>`,
  setup() {
    const items = ref([]); const loading = ref(false); const dlg = ref(false); const editing = ref(false); const saving = ref(false);
    const f = reactive({ id: '', name: '', slug: '', sort_order: 0 });
    const load = async () => { loading.value = true; try { const r = await API.get('/categories'); items.value = r.data; } catch (e) {} loading.value = false; };
    const open = (row) => { editing.value = !!row; if (row) { f.id = row.id; f.name = row.name; f.slug = row.slug; f.sort_order = row.sort_order; } else { f.id = ''; f.name = ''; f.slug = ''; f.sort_order = 0; } dlg.value = true; };
    const save = async () => {
      if (!f.name || !f.slug) return ElMessage.warning('名称和标识必填');
      saving.value = true;
      try {
        if (editing.value) await API.put('/categories/' + f.id, { name: f.name, slug: f.slug, sort_order: f.sort_order });
        else await API.post('/categories', { name: f.name, slug: f.slug, sort_order: f.sort_order });
        ElMessage.success(editing.value ? '已保存' : '创建成功'); dlg.value = false; load();
      } catch (e) { ElMessage.error('操作失败'); }
      saving.value = false;
    };
    const del = async (id) => { try { await API.delete('/categories/' + id); ElMessage.success('已删除'); load(); } catch (e) { ElMessage.error('删除失败'); } };
    onMounted(load);
    return { items, loading, dlg, editing, saving, f, open, save, del };
  }
};

// ── CrawlerTasks ──────────────────────────────────────────────────────────
const CrawlerTasks = {
  template: `
  <div>
    <div class="page-header"><h2>🕷️ 采集任务</h2></div>
    <el-tabs v-model="tab">
      <el-tab-pane label="单本采集" name="single">
        <div class="card">
          <el-radio-group v-model="singleMode" style="margin-bottom:12px"><el-radio-button value="direct">📌 直接采集</el-radio-button><el-radio-button value="search">🔍 搜索匹配</el-radio-button></el-radio-group>
          <div style="display:flex;gap:12px;align-items:center;flex-wrap:wrap">
            <el-select v-model="singleNovelId" filterable placeholder="选择小说" style="width:280px"><el-option v-for="n in allNovels" :key="n.id" :label="n.title+' ('+n.author+')'" :value="n.id"/></el-select>
            <el-input v-model="singleSource" placeholder="规则名" style="width:120px"/>
            <el-button type="primary" @click="triggerSingle" :loading="singleLoading">开始采集</el-button>
          </div>
        </div>
      </el-tab-pane>
      <el-tab-pane label="批量采集" name="batch">
        <div class="card">
          <el-select v-model="batchIds" multiple filterable collapse-tags placeholder="选择多本小说" style="width:100%;margin-bottom:12px"><el-option v-for="n in allNovels" :key="n.id" :label="n.title" :value="n.id"/></el-select>
          <div style="display:flex;gap:12px"><el-input v-model="batchSource" placeholder="规则名" style="width:120px"/><el-button type="primary" @click="triggerBatch" :loading="batchLoading">批量采集({{batchIds.length}})</el-button></div>
        </div>
      </el-tab-pane>
      <el-tab-pane label="书号范围" name="range">
        <div class="card">
          <div style="display:flex;gap:12px;align-items:center;flex-wrap:wrap">
            <el-input v-model="rangeSource" placeholder="规则名" style="width:120px"/>
            <el-input-number v-model="rangeFrom" placeholder="起始ID" :min="1"/>
            <span>—</span><el-input-number v-model="rangeTo" placeholder="结束ID" :min="1"/>
            <el-button type="primary" @click="queueRange">创建并队列采集</el-button>
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>
    <div class="card" style="margin-top:16px">
      <div class="toolbar">
        <el-select v-model="taskStatusFilter" placeholder="状态" clearable style="width:110px" @change="loadTasks"><el-option label="待处理" value="pending"/><el-option label="运行中" value="running"/><el-option label="已完成" value="completed"/><el-option label="失败" value="failed"/></el-select>
        <el-button @click="loadTasks">刷新</el-button>
        <el-button type="danger" :disabled="!taskSelection.length" @click="batchDelTasks">批量删除({{taskSelection.length}})</el-button>
      </div>
      <el-table :data="tasks" stripe v-loading="taskLoading" @selection-change="s=>taskSelection=s.map(r=>r.id)">
        <el-table-column type="selection" width="36"/><el-table-column label="小说" min-width="140"><template #d="r">{{(r.row.novel_id||'').substring(0,8)}}...</template></el-table-column>
        <el-table-column label="状态" width="90"><template #d="r"><el-tag :type="taskStatusType(r.row.status)" size="small">{{r.row.status}}</el-tag></template></el-table-column>
        <el-table-column prop="chapters_found" label="发现" width="60"/><el-table-column prop="chapters_added" label="新增" width="60"/>
        <el-table-column label="错误" min-width="160"><template #d="r"><span style="color:#f56c6c;font-size:12px">{{r.row.error_message?.String||''}}</span></template></el-table-column>
        <el-table-column label="操作" width="220"><template #d="r">
          <el-button link size="small" @click="taskAction(r.row.id,'start')" v-if="r.row.status==='pending'">启动</el-button>
          <el-button link size="small" type="danger" @click="taskAction(r.row.id,'stop')" v-if="r.row.status==='running'">停止</el-button>
          <el-button link size="small" @click="taskAction(r.row.id,'retry')" v-if="r.row.status==='failed'">重试</el-button>
          <el-popconfirm title="确定删除?" @confirm="taskAction(r.row.id,'delete')"><template #reference><el-button link size="small" type="danger">删除</el-button></template></el-popconfirm>
        </template></el-table-column>
      </el-table>
      <div style="margin-top:14px;text-align:right"><el-pagination background layout="total,prev,next" :total="taskTotal" :page-size="20" v-model:current-page="taskPage" @current-change="loadTasks"/></div>
    </div>
  </div>`,
  setup() {
    const tab = ref('single');
    const allNovels = ref([]); const tasks = ref([]); const taskTotal = ref(0); const taskPage = ref(1); const taskLoading = ref(false); const taskStatusFilter = ref(''); const taskSelection = ref([]);
    const singleNovelId = ref(''); const singleSource = ref('23qb'); const singleMode = ref('direct'); const singleLoading = ref(false);
    const batchIds = ref([]); const batchSource = ref('23qb'); const batchLoading = ref(false);
    const rangeSource = ref('23qb'); const rangeFrom = ref(1); const rangeTo = ref(100);
    const taskStatusType = (s) => ({ pending: 'info', running: 'warning', completed: 'success', failed: 'danger' }[s] || 'info');
    const loadNovels = async () => { try { const r = await API.get('/novels', { params: { size: 200 } }); allNovels.value = r.data.items; } catch (e) {} };
    const loadTasks = async () => { taskLoading.value = true; try { const r = await API.get('/crawler/tasks', { params: { page: taskPage.value, status: taskStatusFilter.value || undefined } }); tasks.value = r.data.items; taskTotal.value = r.data.total; } catch (e) {} taskLoading.value = false; };
    const triggerSingle = async () => { if (!singleNovelId.value) return ElMessage.warning('请选择小说'); singleLoading.value = true; try { await API.post('/crawler/trigger', { novel_id: singleNovelId.value, source_name: singleSource.value, mode: singleMode.value }); ElMessage.success('任务已创建'); loadTasks(); } catch (e) { ElMessage.error(e.response?.data?.error || '失败'); } singleLoading.value = false; };
    const triggerBatch = async () => { if (!batchIds.value.length) return; batchLoading.value = true; try { await API.post('/crawler/trigger-batch', { novel_ids: batchIds.value, source_name: batchSource.value }); ElMessage.success('批量任务已创建'); loadTasks(); } catch (e) { ElMessage.error('失败'); } batchLoading.value = false; };
    const queueRange = async () => { try { await API.post('/crawler/tasks/queue-range', { source_name: rangeSource.value, book_from: rangeFrom.value, book_to: rangeTo.value }); ElMessage.success('已创建并排队'); loadTasks(); } catch (e) { ElMessage.error('失败'); } };
    const taskAction = async (id, action) => {
      try {
        if (action === 'delete') { await API.delete('/crawler/tasks/' + id); ElMessage.success('已删除'); }
        else { await API.post('/crawler/tasks/' + id + '/' + action); ElMessage.success('操作成功'); }
        loadTasks();
      } catch (e) { ElMessage.error('操作失败'); }
    };
    const batchDelTasks = async () => { try { await ElMessageBox.confirm('确定删除选中任务?', '警告', { type: 'warning' }); for (const id of taskSelection.value) { try { await API.delete('/crawler/tasks/' + id); } catch (e) {} } ElMessage.success('已删除'); loadTasks(); } catch (e) {} };
    onMounted(() => { loadNovels(); loadTasks(); });
    return { tab, allNovels, tasks, taskTotal, taskPage, taskLoading, taskStatusFilter, taskSelection, singleNovelId, singleSource, singleMode, singleLoading, batchIds, batchSource, batchLoading, rangeSource, rangeFrom, rangeTo, taskStatusType, loadTasks, triggerSingle, triggerBatch, queueRange, taskAction, batchDelTasks };
  }
};

// ── Sites ─────────────────────────────────────────────────────────────────
const Sites = {
  template: `
  <div>
    <div class="page-header"><h2>🌐 站点管理</h2></div>
    <div class="card">
      <div class="toolbar"><div></div><el-button type="primary" @click="open()">+ 新建站点</el-button><el-button type="danger" :disabled="!sel.length" @click="batchDel">批量删除({{sel.length}})</el-button></div>
      <el-table :data="items" stripe v-loading="ld" @selection-change="s=>sel=s.map(r=>r.id)">
        <el-table-column type="selection" width="40"/><el-table-column prop="name" label="名称" width="140"/><el-table-column prop="domain" label="域名" width="200"/>
        <el-table-column prop="template" label="模板" width="90"/><el-table-column prop="language" label="语言" width="70"/>
        <el-table-column label="状态" width="70"><template #d="r"><el-tag :type="r.row.is_active?'success':'danger'" size="small">{{r.row.is_active?'启用':'禁用'}}</el-tag></template></el-table-column>
        <el-table-column label="操作" width="160"><template #d="r">
          <el-button link type="primary" size="small" @click="open(r.row)">编辑</el-button>
          <el-popconfirm title="确定删除?" @confirm="del(r.row.id)"><template #reference><el-button link type="danger" size="small">删除</el-button></template></el-popconfirm>
        </template></el-table-column>
      </el-table>
    </div>
    <el-dialog v-model="dlg" :title="editing?'编辑站点':'新建站点'" width="550px">
      <el-form :model="f" label-width="80px">
        <el-form-item label="名称"><el-input v-model="f.name"/></el-form-item><el-form-item label="域名"><el-input v-model="f.domain"/></el-form-item>
        <el-form-item label="模板"><el-input v-model="f.template"/></el-form-item><el-form-item label="语言"><el-input v-model="f.language"/></el-form-item>
        <el-form-item label="启用"><el-switch v-model="f.is_active"/></el-form-item>
      </el-form>
      <template #footer><el-button @click="dlg=false">取消</el-button><el-button type="primary" @click="save" :loading="saving">{{editing?'保存':'创建'}}</el-button></template>
    </el-dialog>
  </div>`,
  setup() {
    const items = ref([]); const ld = ref(false); const dlg = ref(false); const editing = ref(false); const saving = ref(false); const sel = ref([]);
    const f = reactive({ id: '', name: '', domain: '', template: 'default', language: 'zh', is_active: true });
    const load = async () => { ld.value = true; try { const r = await API.get('/sites'); items.value = r.data; } catch (e) {} ld.value = false; };
    const open = (row) => { editing.value = !!row; Object.assign(f, row || { id: '', name: '', domain: '', template: 'default', language: 'zh', is_active: true }); dlg.value = true; };
    const save = async () => {
      if (!f.name || !f.domain) return ElMessage.warning('名称和域名必填');
      saving.value = true;
      try { if (editing.value) await API.put('/sites/' + f.id, f); else await API.post('/sites', f); ElMessage.success(editing.value ? '已保存' : '创建成功'); dlg.value = false; load(); } catch (e) { ElMessage.error('操作失败'); }
      saving.value = false;
    };
    const del = async (id) => { try { await API.delete('/sites/' + id); ElMessage.success('已删除'); load(); } catch (e) {} };
    const batchDel = async () => { try { await ElMessageBox.confirm('确定删除?', '警告', { type: 'warning' }); for (const id of sel.value) { try { await API.delete('/sites/' + id); } catch (e) {} } ElMessage.success('已删除'); load(); } catch (e) {} };
    onMounted(load);
    return { items, ld, dlg, editing, saving, sel, f, open, save, del, batchDel };
  }
};

// ── LinkRings ─────────────────────────────────────────────────────────────
const LinkRings = {
  template: `
  <div>
    <div class="page-header"><h2>🔗 链轮管理</h2></div>
    <div class="card">
      <div class="toolbar"><div></div><el-button type="primary" @click="openRing()">+ 新建链轮</el-button></div>
      <el-table :data="items" stripe v-loading="ld">
        <el-table-column prop="name" label="名称" width="150"/><el-table-column prop="ring_type" label="类型" width="120"/>
        <el-table-column prop="max_links" label="最大" width="70"/><el-table-column prop="display_mode" label="模式" width="80"/>
        <el-table-column label="目标" width="70"><template #d="r">{{(r.row.targets||[]).length}}</template></el-table-column>
        <el-table-column label="操作" width="200"><template #d="r">
          <el-button link type="primary" size="small" @click="openRing(r.row)">编辑</el-button>
          <el-button link size="small" @click="manageTargets(r.row)">目标({{(r.row.targets||[]).length}})</el-button>
          <el-popconfirm title="确定删除?" @confirm="delRing(r.row.id)"><template #reference><el-button link type="danger" size="small">删除</el-button></template></el-popconfirm>
        </template></el-table-column>
      </el-table>
    </div>
    <el-dialog v-model="ringDlg" :title="ringEditing?'编辑链轮':'新建链轮'" width="500px">
      <el-form :model="rf"><el-form-item label="名称"><el-input v-model="rf.name"/></el-form-item><el-form-item label="类型"><el-select v-model="rf.ring_type"><el-option label="跨站书链" value="cross_site_books"/><el-option label="跨站" value="cross_site"/><el-option label="自定义" value="custom"/></el-select></el-form-item><el-form-item label="最大链接"><el-input-number v-model="rf.max_links" :min="1" :max="50"/></el-form-item><el-form-item label="模式"><el-select v-model="rf.display_mode"><el-option label="侧边栏" value="sidebar"/><el-option label="底部" value="footer"/><el-option label="内联" value="inline"/></el-select></el-form-item></el-form>
      <template #footer><el-button @click="ringDlg=false">取消</el-button><el-button type="primary" @click="saveRing" :loading="ringSaving">{{ringEditing?'保存':'创建'}}</el-button></template>
    </el-dialog>
  </div>`,
  setup() {
    const items = ref([]); const ld = ref(false); const ringDlg = ref(false); const ringEditing = ref(false); const ringSaving = ref(false);
    const rf = reactive({ id: '', name: '', ring_type: 'cross_site', max_links: 10, display_mode: 'sidebar' });
    const load = async () => { ld.value = true; try { const r = await API.get('/link-rings'); items.value = r.data; } catch (e) {} ld.value = false; };
    const openRing = (row) => { ringEditing.value = !!row; Object.assign(rf, row || { id: '', name: '', ring_type: 'cross_site', max_links: 10, display_mode: 'sidebar' }); ringDlg.value = true; };
    const saveRing = async () => { if (!rf.name) return ElMessage.warning('名称必填'); ringSaving.value = true; try { if (ringEditing.value) await API.put('/link-rings/' + rf.id, rf); else await API.post('/link-rings', rf); ElMessage.success(ringEditing.value ? '已保存' : '创建成功'); ringDlg.value = false; load(); } catch (e) { ElMessage.error('失败'); } ringSaving.value = false; };
    const delRing = async (id) => { try { await API.delete('/link-rings/' + id); ElMessage.success('已删除'); load(); } catch (e) {} };
    const manageTargets = (row) => { ElMessage.info('目标管理(功能开发中)'); };
    onMounted(load);
    return { items, ld, ringDlg, ringEditing, ringSaving, rf, load, openRing, saveRing, delRing, manageTargets };
  }
};

// ── Cache ─────────────────────────────────────────────────────────────────
const Cache = {
  template: `
  <div>
    <div class="page-header"><h2>🗄️ 缓存运维</h2></div>
    <div class="stat-grid">
      <div class="stat-card"><div class="icon blue"><el-icon :size="22"><Monitor/></el-icon></div><div><div class="value">{{mem.alloc_mb||0}} MB</div><div class="label">内存占用</div></div></div>
      <div class="stat-card"><div class="icon green"><el-icon :size="22"><Connection/></el-icon></div><div><div class="value">{{mem.goroutines||0}}</div><div class="label">Goroutines</div></div></div>
      <div class="stat-card"><div class="icon orange"><el-icon :size="22"><RefreshRight/></el-icon></div><div><div class="value">{{mem.num_gc||0}}</div><div class="label">GC次数</div></div></div>
    </div>
    <div class="card"><h3>缓存操作</h3><el-button type="primary" @click="flush">刷新缓存</el-button></div>
    <div class="card" style="margin-top:16px"><h3>数据修复</h3><el-descriptions :column="2" border v-if="rs"><el-descriptions-item label="空章节">{{rs.empty_chapters}}</el-descriptions-item><el-descriptions-item label="无封面">{{rs.no_cover}}</el-descriptions-item><el-descriptions-item label="无简介">{{rs.no_description}}</el-descriptions-item><el-descriptions-item label="无作者">{{rs.no_author}}</el-descriptions-item></el-descriptions><el-button type="warning" @click="repair" style="margin-top:12px">修复空章节</el-button></div>
  </div>`,
  setup() {
    const mem = ref({}); const rs = ref(null);
    onMounted(() => { API.get('/cache/health').then(r => mem.value = r.data.memory || {}).catch(() => {}); API.get('/repair/status').then(r => rs.value = r.data).catch(() => {}); });
    const flush = async () => { try { await API.post('/cache/flush'); ElMessage.success('已刷新'); } catch (e) {} };
    const repair = async () => { try { await API.post('/repair/chapters'); ElMessage.success('已启动'); } catch (e) { ElMessage.warning('Go版开发中'); } };
    return { mem, rs, flush, repair };
  }
};

// ── Router ────────────────────────────────────────────────────────────────
const routes = [
  { path: '/', redirect: '/dashboard' },
  { path: '/login', component: LoginPage, meta: { title: '登录' } },
  { path: '/dashboard', component: Dashboard, meta: { title: '控制台' } },
  { path: '/novels', component: NovelList, meta: { title: '小说管理' } },
  { path: '/novels/create', component: NovelForm, props: { id: 'create' }, meta: { title: '新建小说' } },
  { path: '/novels/:id', component: NovelDetailPage, props: true, meta: { title: '小说详情' } },
  { path: '/novels/:id/edit', component: NovelForm, props: true, meta: { title: '编辑小说' } },
  { path: '/novels/:novelId/chapters', component: ChapterList, props: true, meta: { title: '章节管理' } },
  { path: '/novels/:novelId/chapters/:chapterId', component: ChapterEditor, props: true, meta: { title: '章节编辑' } },
  { path: '/categories', component: CategoriesC, meta: { title: '分类管理' } },
  { path: '/crawler', component: CrawlerTasks, meta: { title: '采集任务' } },
  { path: '/sites', component: Sites, meta: { title: '站点管理' } },
  { path: '/link-rings', component: LinkRings, meta: { title: '链轮管理' } },
  { path: '/cache', component: Cache, meta: { title: '缓存运维' } },
];
const router = createRouter({ history: createWebHashHistory(), routes });
router.beforeEach((to, from) => { if (to.path !== '/login' && !atok()) return '/login'; if (to.path === '/login' && atok()) return '/dashboard'; });

// ── App ───────────────────────────────────────────────────────────────────
const AppC = { template: '<router-view/>' };
const app = createApp(AppC);
app.use(router); app.use(ElementPlus);
for (const [k, v] of Object.entries(Icons)) app.component(k, v);
app.mount('#app');
