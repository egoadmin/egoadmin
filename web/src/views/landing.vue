<script lang="ts" setup name="Landing">
import { onBeforeUnmount, onMounted, ref } from 'vue'

const router = useRouter()

const title = import.meta.env.VITE_APP_TITLE

const rootRef = ref<HTMLElement>()
const activeSection = ref('')
let revealObserver: IntersectionObserver | null = null
let sectionObserver: IntersectionObserver | null = null

const navLinks = [
  { id: 'abilities', label: '设计能力' },
  { id: 'pipeline', label: '开发流水线' },
  { id: 'quickstart', label: '快速开始' },
  { id: 'explore', label: '继续探索' },
]

const commands = [
  'make install',
  'make dev-up',
  'make gen',
  'make run',
  'make e2e',
  'make migrate.new SERVICE=user NAME=add_field',
]
const copiedIndex = ref(-1)

function scrollToSection(id: string) {
  const el = rootRef.value?.querySelector(`#${id}`)
  el?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

function copyCommand(cmd: string, index: number) {
  navigator.clipboard?.writeText(cmd).catch(() => {})
  copiedIndex.value = index
  window.setTimeout(() => {
    if (copiedIndex.value === index) {
      copiedIndex.value = -1
    }
  }, 1400)
}

function goExplore(path: string) {
  router.push(path)
}

onMounted(() => {
  const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  const root = rootRef.value
  if (!root) {
    return
  }
  // 滚动渐显
  const revealEls = Array.from(root.querySelectorAll<HTMLElement>('.rv'))
  if (reduced) {
    revealEls.forEach(el => el.classList.add('in'))
  } else {
    revealObserver = new IntersectionObserver((entries) => {
      entries.forEach((entry) => {
        if (entry.isIntersecting) {
          entry.target.classList.add('in')
          revealObserver?.unobserve(entry.target)
        }
      })
    }, { threshold: 0.12, rootMargin: '0px 0px -8% 0px' })
    revealEls.forEach((el, i) => {
      el.style.transitionDelay = `${Math.min(i, 6) * 40}ms`
      revealObserver?.observe(el)
    })
  }
  // 导航高亮
  const sections = navLinks
    .map(l => root.querySelector(`#${l.id}`))
    .filter((el): el is Element => !!el)
  sectionObserver = new IntersectionObserver((entries) => {
    entries.forEach((entry) => {
      if (entry.isIntersecting) {
        activeSection.value = entry.target.id
      }
    })
  }, { rootMargin: '-45% 0px -50% 0px' })
  sections.forEach(s => sectionObserver?.observe(s))
})

onBeforeUnmount(() => {
  revealObserver?.disconnect()
  sectionObserver?.disconnect()
})
</script>

<template>
  <div ref="rootRef" class="landing">
    <!-- ============================ NAV ============================ -->
    <nav class="lnav">
      <div class="brand">
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="var(--accent)" stroke-width="1.6">
          <path d="M12 2v20M2 12h20M5 5l14 14M19 5L5 19" stroke-linecap="round" />
        </svg>
        {{ title }} <small class="mono">EgoAdmin</small>
      </div>
      <div class="links">
        <a
          v-for="link in navLinks"
          :key="link.id"
          :class="{ on: activeSection === link.id }"
          @click="scrollToSection(link.id)"
        >{{ link.label }}</a>
      </div>
      <div class="spacer" />
      <button class="btn btn-secondary" @click="scrollToSection('quickstart')">文档</button>
      <button class="btn btn-primary" @click="goExplore('/system/role')">进入控制台</button>
    </nav>

    <!-- ============================ HERO ============================ -->
    <header class="hero">
      <div class="grid-bg" />
      <div class="wrap inner">
        <div>
          <div class="eyebrow mono">READY-TO-RUN · GO 微服务后台开发底座</div>
          <h1>少写后台基础设施，<br>把业务系统快速跑起来</h1>
          <p class="lede">
            EgoAdmin 不是只带登录和 CRUD 的后台模板，而是一套面向 Go 微服务的开发底座。它用 Proto First 串起接口、验证、文档、权限和响应映射，同时沉淀统一文件能力、契约化权限、分布式 ID、认证会话、数据库迁移和 AI 友好的工程规则。
          </p>
          <div class="cta">
            <button class="btn btn-primary btn-lg" @click="scrollToSection('quickstart')">快速开始</button>
            <button class="btn btn-secondary btn-lg" @click="scrollToSection('abilities')">查看设计能力</button>
          </div>
          <div class="meta mono">
            默认服务 <b>gateway · user · idgen</b>　│　存储 <b>MySQL · Redis · MinIO</b>
          </div>
        </div>
        <div class="codewin">
          <div class="bar">
            <span class="dot r" /><span class="dot y" /><span class="dot g" />
            <span class="file mono">api/proto/user/v1/role.proto</span>
            <span class="badge mono">Proto First</span>
          </div>
          <pre class="mono"><span class="c">// 定义 proto，就完成半个接口工程</span>
<span class="k">service</span> <span class="t">RoleService</span> {
  <span class="k">rpc</span> AddRole(AddRoleRequest)
      <span class="k">returns</span> (AddRoleResponse) {
<span class="hl">    option (google.api.http) = { post: <span class="s">"/user.v1.RoleService/AddRole"</span> };</span>
  }
}
<span class="k">message</span> <span class="t">Role</span> {
  <span class="k">string</span> name = 1 [(validate) = <span class="s">"required"</span>, (label) = <span class="s">"角色名称"</span>];<span class="caret" />
}
<span class="c">→ 继续生成 gRPC / HTTP 双栈 · 校验 · OpenAPI · 权限标识</span></pre>
        </div>
      </div>
    </header>

    <!-- ============================ CAPABILITY STRIP ============================ -->
    <section class="cap-section">
      <div class="wrap">
        <div class="cap rv">
          <div><div class="lbl">开发契约</div><div class="val mono">Proto First</div></div>
          <div><div class="lbl">协议入口</div><div class="val mono">HTTP + gRPC</div></div>
          <div><div class="lbl">默认服务</div><div class="val mono">gateway·user·idgen</div></div>
          <div><div class="lbl">基础存储</div><div class="val mono">MySQL·Redis·MinIO</div></div>
        </div>
      </div>
    </section>

    <!-- ============================ PROPOSITIONS ============================ -->
    <section class="section">
      <div class="wrap">
        <div class="props">
          <div class="prop rv">
            <h3>定义 proto，就完成半个接口工程</h3>
            <p>service、rpc、request、response 和字段标签会继续生成 HTTP/gRPC 双栈入口、参数验证、OpenAPI 文档、API manifest、权限标识和响应映射依据。</p>
          </div>
          <div class="prop rv">
            <h3>文件能力不是一个上传接口</h3>
            <p>统一处理 multipart、TUS 断点续传、秒传、对象存储、file_reference、Range 下载和图片动态处理，业务只需要绑定 referenceId。</p>
          </div>
          <div class="prop rv">
            <h3>权限简单，但边界完整</h3>
            <p>接口契约、routeMenu、permission-contract、Casbin 和角色授权形成闭环，开发前中期不用在业务代码里到处补权限胶水。</p>
          </div>
        </div>
      </div>
    </section>

    <!-- ============================ DESIGN ABILITIES ============================ -->
    <section id="abilities" class="section">
      <div class="wrap">
        <div class="sec-head rv">
          <div class="eyebrow mono">设计能力</div>
          <h2>EgoAdmin 提前沉淀的后台系统基础设施</h2>
          <p>把派生项目里反复重写、最容易出错的底层能力，固化为契约驱动、边界清晰的工程底座。</p>
        </div>
        <div class="feats">
          <div class="feat rv">
            <div class="k mono">CONTRACT</div>
            <h3>Proto First 契约驱动</h3>
            <p>业务接口从 proto 开始，而不是从临时 HTTP handler 或页面按钮开始，减少多处重复定义造成的不一致。</p>
            <ul>
              <li>同一 RPC 同时生成 <span class="mono">gRPC</span> 服务和 <span class="mono">POST HTTP</span> 兼容路径</li>
              <li><span class="mono">validate</span>、<span class="mono">label</span>、<span class="mono">field_behavior</span> 和 OpenAPI 注解共同描述请求约束</li>
              <li><span class="mono">copier</span> 标签和转换方法明确 ORM/model 到响应结构的映射</li>
            </ul>
          </div>
          <div class="feat rv">
            <div class="k mono">FILE</div>
            <h3>统一上传、下载和图片处理</h3>
            <p>把后台项目最容易散落的文件生命周期收进 gateway 和 upload/cdn 组件。</p>
            <ul>
              <li>multipart、TUS 断点续传、秒传预检和对象存储直写</li>
              <li><span class="mono">/cdn/file/</span> 支持认证、签名、inline 和 Range 下载</li>
              <li><span class="mono">file_object</span>、<span class="mono">file_reference</span>、<span class="mono">upload_session</span> 管理临时、绑定、释放和清理</li>
            </ul>
          </div>
          <div class="feat rv">
            <div class="k mono">RBAC</div>
            <h3>契约化权限和数据范围</h3>
            <p>权限不靠散落的字符串和人工记忆维护，而是由接口、菜单、按钮和角色授权链路约束。</p>
            <ul>
              <li>API catalog、routeMenu 和 permission-contract 约束角色可授权边界</li>
              <li><span class="mono">DataScope</span> 支持全部、本组织及以下、本组织、仅本人等数据范围</li>
              <li>普通管理员不能创建或授予超出自身范围的角色权限</li>
            </ul>
          </div>
          <div class="feat rv">
            <div class="k mono">IDGEN</div>
            <h3>稳定高效的分布式 ID</h3>
            <p>内置 idgen 服务，避免每个派生项目重复处理 Snowflake、号段、机器号和 ID 外露问题。</p>
            <ul>
              <li>号段模式提供高性能全局 ID 分配</li>
              <li>机器号租约和关闭释放适合多服务部署</li>
              <li><span class="mono">idcodec</span> 支持内部数字 ID 到外部字符串 ID 的可逆编码</li>
            </ul>
          </div>
          <div class="feat rv">
            <div class="k mono">BASE</div>
            <h3>微服务工程底座</h3>
            <p>默认 gateway、user、idgen 三个服务，保留真实微服务项目需要的启动、治理和迁移链路。</p>
            <ul>
              <li>gateway 统一承接 HTTP 兼容、前端内嵌、上传下载和 API 聚合</li>
              <li><span class="mono">Atlas + GORM</span> model 按服务数据库边界生成版本化迁移</li>
              <li>readiness、health check、graceful shutdown 和资源关闭顺序已内置</li>
            </ul>
          </div>
          <div class="feat rv">
            <div class="k mono">AI</div>
            <h3>AI 编程友好</h3>
            <p>项目把容易出错的开发顺序写成稳定规则，让 AI 更容易按正确层次扩展业务。</p>
            <ul>
              <li>EgoAdmin skill 固化 proto、权限、迁移、上传和验证规则</li>
              <li>新增能力按 <span class="mono">proto → 生成 → 实现 → 权限 → 验证</span> 推进</li>
              <li><span class="mono">make gen</span>、前端 build、service check 和 e2e 让改动可验证</li>
            </ul>
          </div>
        </div>
      </div>
    </section>

    <!-- ============================ PIPELINE ============================ -->
    <section id="pipeline" class="section section-alt">
      <div class="wrap">
        <div class="sec-head rv">
          <div class="eyebrow mono">开发流水线</div>
          <h2>新增业务能力时的默认路径</h2>
          <p>每一步都落在正确的层，改动始终可生成、可授权、可验证。</p>
        </div>
        <div class="pipe">
          <div class="step rv"><b class="mono">定义 proto</b><p>接口、请求响应、校验标签、OpenAPI 说明和 copier 映射一起设计。</p></div>
          <div class="step rv"><b class="mono">make gen</b><p>生成 gRPC/HTTP 兼容代码、OpenAPI、API catalog 和前端 API manifest。</p></div>
          <div class="step rv"><b class="mono">实现三层</b><p>controller、service、store：业务规则、事务、数据权限和响应转换留在正确层。</p></div>
          <div class="step rv"><b class="mono">绑定 routeMenu</b><p>页面、按钮和 API 权限进入 permission-contract，角色授权受契约约束。</p></div>
          <div class="step rv"><b class="mono">运行验证</b><p>前端构建、服务测试、迁移校验和 gateway e2e 覆盖完整链路。</p></div>
        </div>
      </div>
    </section>

    <!-- ============================ QUICKSTART ============================ -->
    <section id="quickstart" class="section">
      <div class="wrap">
        <div class="qs">
          <div class="rv">
            <div class="eyebrow mono">快速开始</div>
            <h2>本地开发常用命令</h2>
            <p class="qs-lede">从安装依赖到端到端验证，一组 make 命令贯穿日常开发循环。点击任意命令即可复制。</p>
            <button class="btn btn-primary btn-lg" @click="scrollToSection('explore')">查看后台能力</button>
          </div>
          <div class="term rv">
            <div class="bar">
              <span class="dot r" /><span class="dot y" /><span class="dot g" />
              <span class="file mono">~/egoadmin — make</span>
            </div>
            <div class="term-body">
              <div
                v-for="(cmd, i) in commands"
                :key="cmd"
                class="cmd"
                :class="{ done: copiedIndex === i }"
                @click="copyCommand(cmd, i)"
              >
                <span class="pr mono">$</span>
                <code class="mono">{{ cmd }}</code>
                <span class="cp mono">{{ copiedIndex === i ? '已复制 ✓' : '复制' }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- ============================ EXPLORE ============================ -->
    <section id="explore" class="section section-alt">
      <div class="wrap">
        <div class="sec-head rv">
          <div class="eyebrow mono">继续探索</div>
          <h2>从这些后台能力开始查看</h2>
          <p>这些是底座默认带起的后台模块，体现契约化权限与数据范围在真实页面中的样子。</p>
        </div>
        <div class="explore">
          <a class="ecard rv" @click="goExplore('/system/role')">
            <h3>角色管理 <span class="arr">→</span></h3>
            <p>查看菜单、API 和数据权限如何授权。</p>
            <div class="path mono">/system/role</div>
          </a>
          <a class="ecard rv" @click="goExplore('/user_management/user')">
            <h3>用户管理 <span class="arr">→</span></h3>
            <p>查看账号、角色和组织关系。</p>
            <div class="path mono">/user_management/user</div>
          </a>
          <a class="ecard rv" @click="goExplore('/user_management/organization')">
            <h3>组织管理 <span class="arr">→</span></h3>
            <p>查看组织树和数据范围基础。</p>
            <div class="path mono">/user_management/organization</div>
          </a>
          <a class="ecard rv" @click="goExplore('/system/operation_log')">
            <h3>操作日志 <span class="arr">→</span></h3>
            <p>查看网关到用户服务的操作审计。</p>
            <div class="path mono">/system/operation_log</div>
          </a>
        </div>
      </div>
    </section>

    <!-- ============================ FOOTER ============================ -->
    <footer class="lfooter">
      <div class="wrap finner">
        <div>EgoAdmin · 开箱即用的微服务后台开发底座</div>
        <div class="flinks">
          <a @click="scrollToSection('abilities')">设计能力</a>
          <a @click="scrollToSection('pipeline')">开发流水线</a>
          <a @click="scrollToSection('quickstart')">快速开始</a>
          <a @click="scrollToSection('explore')">继续探索</a>
        </div>
      </div>
    </footer>
  </div>
</template>

<style lang="scss" scoped>
.landing {
  min-height: 100vh;
  font-size: 15px;
  line-height: 1.65;
  color: var(--text);
  background: var(--bg);

  --maxw: 1180px;
}

.wrap {
  max-width: var(--maxw);
  margin: 0 auto;
  padding: 0 28px;
}

.section {
  padding: 88px 0;
}

.section-alt {
  background: var(--surface-2);
  border-top: 1px solid var(--border);
  border-bottom: 1px solid var(--border);
}

h1, h2, h3, h4 {
  margin: 0;
  font-family: var(--font-display);
  color: var(--fg);
  text-wrap: balance;
}

.eyebrow {
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.09em;
  text-transform: uppercase;
  color: var(--accent-text);
}

.sec-head {
  max-width: 64ch;
  margin-bottom: 44px;

  h2 {
    margin: 10px 0 12px;
    font-size: clamp(28px, 3vw, 38px);
    font-weight: 660;
    line-height: 1.14;
    letter-spacing: -0.018em;
  }

  p {
    margin: 0;
    font-size: 16px;
    color: var(--muted);
  }
}

/* ---------- buttons ---------- */
.btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 9px 18px;
  font: inherit;
  font-size: 14px;
  font-weight: 550;
  cursor: pointer;
  border: 1px solid transparent;
  border-radius: var(--r-md);
  transition: background 0.18s var(--ease), border-color 0.18s var(--ease), color 0.18s var(--ease);
}

.btn-primary {
  color: var(--on-accent);
  background: var(--accent);

  &:hover { background: var(--accent-hover); }
  &:active { background: var(--accent-press); }
}

.btn-secondary {
  color: var(--text);
  background: var(--surface);
  border-color: var(--border-strong);

  &:hover { color: var(--accent-text); border-color: var(--accent); }
}

.btn-lg {
  padding: 12px 24px;
  font-size: 16px;
}

/* ---------- top nav ---------- */
.lnav {
  position: sticky;
  top: 0;
  z-index: 100;
  display: flex;
  align-items: center;
  gap: 32px;
  height: 64px;
  padding: 0 28px;
  background: color-mix(in oklab, var(--bg) 82%, transparent);
  backdrop-filter: saturate(1.4) blur(10px);
  border-bottom: 1px solid var(--border);

  .brand {
    display: flex;
    align-items: center;
    gap: 10px;
    font-family: var(--font-display);
    font-size: 16px;
    font-weight: 680;
    color: var(--fg);
    letter-spacing: -0.01em;

    small {
      font-size: 11px;
      font-weight: 500;
      color: var(--muted);
    }
  }

  .links {
    display: flex;
    gap: 6px;
    margin-left: 6px;

    a {
      padding: 8px 12px;
      font-size: 14px;
      font-weight: 500;
      color: var(--muted);
      cursor: pointer;
      border-radius: var(--r-md);
      transition: color 0.18s var(--ease), background 0.18s var(--ease);

      &:hover { color: var(--fg); background: var(--surface-2); }
      &.on { color: var(--accent-text); }
    }
  }

  .spacer { flex: 1; }
}

/* ---------- hero ---------- */
.hero {
  position: relative;
  padding: 80px 0 64px;
  overflow: hidden;

  .grid-bg {
    position: absolute;
    inset: -40px 0 auto 0;
    z-index: 0;
    height: 520px;
    background-image:
      linear-gradient(var(--border) 1px, transparent 1px),
      linear-gradient(90deg, var(--border) 1px, transparent 1px);
    background-size: 34px 34px;
    opacity: 0.6;
    mask-image: radial-gradient(120% 70% at 22% 0%, #000, transparent 72%);
    -webkit-mask-image: radial-gradient(120% 70% at 22% 0%, #000, transparent 72%);
  }

  .inner {
    position: relative;
    z-index: 1;
    display: grid;
    grid-template-columns: 1.04fr 0.96fr;
    gap: 52px;
    align-items: center;
  }

  h1 {
    margin: 16px 0 18px;
    font-size: clamp(38px, 4.6vw, 60px);
    font-weight: 680;
    line-height: 1.05;
    color: var(--fg);
    letter-spacing: -0.024em;
  }

  .lede {
    max-width: 48ch;
    font-size: 17px;
    color: var(--muted);
  }

  .cta {
    display: flex;
    flex-wrap: wrap;
    gap: 12px;
    margin-top: 28px;
  }

  .meta {
    margin-top: 22px;
    font-size: 12.5px;
    color: var(--faint);

    b { font-weight: 600; color: var(--text); }
  }
}

/* ---------- code window ---------- */
.codewin {
  overflow: hidden;
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: var(--r-xl);
  box-shadow: var(--shadow-md);

  .bar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 13px 16px;
    border-bottom: 1px solid var(--border);
  }

  .dot {
    width: 11px;
    height: 11px;
    border-radius: 50%;

    &.r { background: #ed6a5e; }
    &.y { background: #f5c14e; }
    &.g { background: #61c554; }
  }

  .file {
    margin-left: 6px;
    font-size: 12.5px;
    color: var(--muted);
  }

  .badge {
    margin-left: auto;
    padding: 3px 9px;
    font-size: 11px;
    font-weight: 600;
    color: var(--cyan);
    background: var(--cyan-weak);
    border-radius: var(--r-pill);
  }

  pre {
    margin: 0;
    padding: 20px;
    overflow: auto;
    font-size: 13px;
    line-height: 1.75;
    color: var(--text);
  }

  .hl {
    display: block;
    margin: 0 -20px;
    padding: 0 20px;
    background: linear-gradient(90deg, var(--cyan-weak), transparent);
  }

  .k { font-weight: 600; color: var(--accent-text); }
  .s { color: var(--success); }
  .c { color: var(--faint); }
  .t { color: var(--cyan); }

  .caret {
    display: inline-block;
    width: 8px;
    height: 15px;
    vertical-align: -2px;
    background: var(--accent);
    animation: blink 1.1s steps(1) infinite;
  }
}

@keyframes blink {
  50% { opacity: 0; }
}

/* ---------- capability strip ---------- */
.cap-section {
  padding-top: 24px;
}

.cap {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 1px;
  overflow: hidden;
  background: var(--border);
  border: 1px solid var(--border);
  border-radius: var(--r-lg);

  > div {
    padding: 20px 22px;
    background: var(--surface);
  }

  .lbl { font-size: 13px; color: var(--muted); }

  .val {
    margin-top: 6px;
    font-size: 16px;
    font-weight: 600;
    color: var(--fg);
    letter-spacing: -0.01em;
  }
}

/* ---------- propositions ---------- */
.props {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 28px;
}

.prop {
  padding-top: 22px;
  border-top: 2px solid var(--accent);

  &:nth-child(2) { border-color: var(--cyan); }

  h3 { margin-bottom: 10px; font-size: 19px; font-weight: 620; }
  p { margin: 0; font-size: 14.5px; color: var(--muted); }
}

/* ---------- feature cards ---------- */
.feats {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 18px;
}

.feat {
  padding: 24px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--r-lg);
  transition: border-color 0.18s var(--ease), box-shadow 0.18s var(--ease), transform 0.18s var(--ease);

  &:hover {
    border-color: var(--border-strong);
    box-shadow: var(--shadow-sm);
    transform: translateY(-2px);
  }

  .k {
    font-size: 11px;
    font-weight: 600;
    color: var(--cyan);
    letter-spacing: 0.06em;
  }

  h3 { margin: 10px 0 8px; font-size: 18px; font-weight: 620; }

  > p { margin: 0 0 16px; font-size: 14px; color: var(--muted); }

  ul {
    display: flex;
    flex-direction: column;
    gap: 9px;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  li {
    position: relative;
    padding-left: 20px;
    font-size: 13px;
    line-height: 1.55;
    color: var(--text);

    &::before {
      content: "";
      position: absolute;
      top: 8px;
      left: 2px;
      width: 6px;
      height: 6px;
      background: var(--border-strong);
      border-radius: 1px;
      transform: rotate(45deg);
    }

    .mono { font-weight: 500; color: var(--text); }
  }
}

/* ---------- pipeline ---------- */
.pipe {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  counter-reset: s;
}

.pipe .step {
  position: relative;
  padding: 0 18px;

  &::before {
    counter-increment: s;
    content: counter(s);
    display: flex;
    align-items: center;
    justify-content: center;
    width: 34px;
    height: 34px;
    margin-bottom: 16px;
    font-size: 14px;
    font-weight: 600;
    color: var(--accent-text);
    background: var(--surface);
    border: 1.5px solid var(--accent);
    border-radius: 50%;
  }

  &::after {
    content: "";
    position: absolute;
    top: 16px;
    left: 52px;
    right: -1px;
    height: 2px;
    background: linear-gradient(90deg, var(--cyan), var(--cyan-weak));
    opacity: 0.7;
  }

  &:last-child::after { display: none; }

  b { display: block; margin-bottom: 6px; font-size: 14px; color: var(--fg); }
  p { margin: 0; font-size: 13px; color: var(--muted); }
}

/* ---------- quickstart ---------- */
.qs {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 40px;
  align-items: center;

  h2 {
    margin: 10px 0 12px;
    font-size: clamp(28px, 3vw, 38px);
    font-weight: 660;
    letter-spacing: -0.018em;
  }

  .qs-lede {
    margin: 0 0 20px;
    font-size: 16px;
    color: var(--muted);
  }
}

.term {
  overflow: hidden;
  background: #0e1116;
  border: 1px solid #20262f;
  border-radius: var(--r-xl);
  box-shadow: var(--shadow-lg);

  .bar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 13px 16px;
    border-bottom: 1px solid #20262f;
  }

  .dot {
    width: 11px;
    height: 11px;
    border-radius: 50%;

    &.r { background: #ed6a5e; }
    &.y { background: #f5c14e; }
    &.g { background: #61c554; }
  }

  .file { margin-left: 6px; font-size: 12px; color: #8b95a4; }

  .term-body { padding: 8px 6px; }

  .cmd {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 14px;
    cursor: pointer;
    border-radius: var(--r-md);
    transition: background 0.15s var(--ease);

    &:hover { background: #1a212b; }

    .pr { font-size: 13px; color: var(--cyan); }

    code {
      flex: 1;
      overflow: hidden;
      font-size: 13px;
      color: #e6eaf0;
      white-space: nowrap;
      text-overflow: ellipsis;
    }

    .cp {
      font-size: 11px;
      color: #6a7382;
      opacity: 0;
      transition: opacity 0.15s var(--ease);
    }

    &:hover .cp { opacity: 1; }

    &.done .cp { color: var(--success); opacity: 1; }
  }
}

/* ---------- explore ---------- */
.explore {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 18px;
}

.ecard {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 22px;
  cursor: pointer;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--r-lg);
  transition: border-color 0.18s var(--ease), box-shadow 0.18s var(--ease), transform 0.18s var(--ease);

  &:hover {
    border-color: var(--accent);
    box-shadow: var(--shadow-sm);
    transform: translateY(-2px);
  }

  h3 {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 17px;
    font-weight: 620;
    color: var(--fg);

    .arr { color: var(--accent); transition: transform 0.18s var(--ease); }
  }

  &:hover h3 .arr { transform: translateX(3px); }

  p { margin: 0; font-size: 13px; color: var(--muted); }

  .path {
    margin-top: auto;
    padding-top: 8px;
    font-size: 11px;
    color: var(--faint);
  }
}

/* ---------- footer ---------- */
.lfooter {
  padding: 40px 0;
  font-size: 13px;
  color: var(--muted);
  border-top: 1px solid var(--border);

  .finner {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 20px;
  }

  .flinks {
    display: flex;
    gap: 20px;

    a { color: var(--muted); cursor: pointer; }
    a:hover { color: var(--fg); }
  }
}

/* ---------- reveal ---------- */
.rv {
  opacity: 0;
  transform: translateY(14px);
  transition: opacity 0.5s var(--ease-out), transform 0.5s var(--ease-out);

  &.in { opacity: 1; transform: none; }
}

/* ---------- responsive ---------- */
@media (max-width: 920px) {
  .hero .inner { grid-template-columns: 1fr; gap: 36px; }
  .qs { grid-template-columns: 1fr; gap: 28px; }
  .feats { grid-template-columns: 1fr 1fr; }
  .pipe { grid-template-columns: 1fr 1fr; gap: 24px; }
  .pipe .step::after { display: none; }
  .explore { grid-template-columns: 1fr 1fr; }
  .lnav .links { display: none; }
}

@media (max-width: 600px) {
  .section { padding: 60px 0; }
  .cap { grid-template-columns: 1fr 1fr; }
  .props, .feats, .explore, .pipe { grid-template-columns: 1fr; }
}

@media (prefers-reduced-motion: reduce) {
  .rv { opacity: 1; transform: none; }
  .codewin .caret { animation: none; }
}
</style>
