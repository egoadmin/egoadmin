<route lang="yaml">
meta:
  title: app.login
  constant: true
  layout: false
</route>

<script lang="ts" setup name="Login">
import type { FormInstance, FormRules } from 'element-plus'
import { ElMessage } from 'element-plus'
import Fingerprint2 from 'fingerprintjs2'
import useUserStore from '@/store/modules/user'
import { t } from '@/i18n'

const route = useRoute()
const router = useRouter()

const userStore = useUserStore()

const title = import.meta.env.VITE_APP_TITLE

// 表单类型，login 登录，reset 重置密码
const formType = ref('login')
const loading = ref(false)
const redirect = ref(route.query.redirect?.toString() ?? '/')

// 登录
const loginFormRef = ref<FormInstance>()
const loginForm = ref({
  username: localStorage.login_username || '',
  password: '',
  remember: !!localStorage.login_username,
})
const loginRules = ref<FormRules>({
  username: [{ required: true, trigger: 'blur', message: t('login.usernameRequired') }],
  password: [
    { required: true, trigger: 'blur', message: t('login.passwordRequired') },
    { min: 6, max: 20, trigger: 'blur', message: t('login.passwordLength') },
  ],
})

const ua = ref('')
// 创建浏览器指纹
function createFingerprint() {
  return new Promise<string>((resolve) => {
    Fingerprint2.get((components: any) => {
      // 参数只有回调函数时，默认浏览器指纹依据所有配置信息进行生成
      const values = components.map((component: any) => component.value) // 配置的值的数组
      ua.value = Fingerprint2.x64hash128(values.join(''), 31) // 生成浏览器指纹
      resolve(ua.value)
    })
  })
}
function handleLogin() {
  if (!loginFormRef.value) {
    return
  }
  loginFormRef.value.validate(async (valid) => {
    if (valid) {
      const { password, username } = loginForm.value
      loading.value = true
      const currentUA = ua.value || (await createFingerprint())
      userStore
        .login({ username, password, ua: currentUA })
        .then(() => {
          loading.value = false
          if (loginForm.value.remember) {
            localStorage.setItem('login_username', loginForm.value.username)
          } else {
            localStorage.removeItem('login_username')
          }
          router.push('/')
          console.log(redirect.value)
        })
        .catch(() => {
          loading.value = false
        })
    }
  })
}
createFingerprint()
// 注册
const registerFormRef = ref<FormInstance>()
const registerForm = ref({
  username: '',
  captcha: '',
  password: '',
  checkPassword: '',
})
const registerRules = ref<FormRules>({
  username: [{ required: true, trigger: 'blur', message: t('login.usernameRequired') }],
  captcha: [{ required: true, trigger: 'blur', message: t('login.captchaRequired') }],
  password: [
    { required: true, trigger: 'blur', message: t('login.passwordRequired') },
    { min: 6, max: 20, trigger: 'blur', message: t('login.passwordLength') },
  ],
  checkPassword: [
    { required: true, trigger: 'blur', message: t('login.confirmPasswordRequired') },
    {
      validator: (rule, value, callback) => {
        if (value !== registerForm.value.password) {
          callback(new Error(t('login.passwordMismatch')))
        } else {
          callback()
        }
      },
    },
  ],
})
function handleRegister() {
  ElMessage({
    message: t('login.registerNotice'),
    type: 'warning',
  })
  if (!registerFormRef.value) {
    return
  }
  registerFormRef.value.validate((valid) => {
    if (valid) {
      // 这里编写业务代码
    }
  })
}

// 重置密码
const resetFormRef = ref<FormInstance>()
const resetForm = ref({
  username: localStorage.login_username || '',
  captcha: '',
  newPassword: '',
})
const resetRules = ref<FormRules>({
  username: [{ required: true, trigger: 'blur', message: t('login.usernameRequired') }],
  captcha: [{ required: true, trigger: 'blur', message: t('login.captchaRequired') }],
  newPassword: [
    { required: true, trigger: 'blur', message: t('login.newPasswordRequired') },
    { min: 6, max: 20, trigger: 'blur', message: t('login.passwordLength') },
  ],
})
function handleReset() {
  ElMessage({
    message: t('login.resetNotice'),
    type: 'warning',
  })
  if (!resetFormRef.value) {
    return
  }
  resetFormRef.value.validate((valid) => {
    if (valid) {
      // 这里编写业务代码
    }
  })
}

function testAccount(username: string) {
  loginForm.value.username = username
  loginForm.value.password = '123456'
  handleLogin()
}
</script>

<template>
  <div class="login-page">
    <!-- 左：契约面板（固定深色，不随主题） -->
    <aside class="brand-panel">
      <div class="grid-bg" />
      <div class="bp-logo">
        <svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="1.6">
          <path d="M12 2v20M2 12h20M5 5l14 14M19 5L5 19" stroke-linecap="round" />
        </svg>
        {{ title }} <small class="mono">EgoAdmin</small>
      </div>
      <div class="bp-mid">
        <div class="bp-eyebrow mono">GO 微服务后台开发底座</div>
        <h2>这是给工程师的底座，<br>不是又一个后台模板</h2>
        <p>Proto First 串起接口、验证、文档、权限与响应映射。登录后即可查看角色、用户、组织与操作审计如何由契约约束。</p>
        <div class="bp-code">
          <div class="bar">
            <span class="dot" style="background: #ed6a5e" />
            <span class="dot" style="background: #f5c14e" />
            <span class="dot" style="background: #61c554" />
            <span class="file mono">make run</span>
          </div>
          <pre class="mono"><span class="c"># 启动默认服务</span>
<span class="k">$</span> make run
  <span class="t">gateway</span>  listening :8080  <span class="c">(HTTP+gRPC)</span>
  <span class="t">user</span>     ready  · casbin loaded
  <span class="t">idgen</span>    ready  · segment mode
<span class="c"># readiness / graceful shutdown 已内置</span></pre>
        </div>
        <div class="bp-chips mono">
          <span>Proto First</span><span>HTTP + gRPC</span><span>gateway·user·idgen</span>
        </div>
      </div>
      <div class="bp-foot mono">EgoAdmin · 开箱即用的微服务后台开发底座</div>
    </aside>

    <!-- 右：登录卡 -->
    <main class="form-panel">
      <el-form
        v-show="formType === 'login'"
        ref="loginFormRef"
        :model="loginForm"
        :rules="loginRules"
        class="login-form"
        autocomplete="on"
      >
        <div class="title-container">
          <h3 class="title">{{ t('login.title', { name: title }) }}</h3>
          <p class="sub">{{ t('login.subtitle') }}</p>
        </div>
        <div>
          <el-form-item prop="username" class="labeled-field">
            <label class="field-label">{{ t('login.username') }}</label>
            <el-input
              v-model="loginForm.username"
              :placeholder="t('login.usernamePlaceholder')"
              text
              tabindex="1"
              autocomplete="on"
              maxlength="20"
            />
          </el-form-item>
          <el-form-item prop="password" class="labeled-field">
            <label class="field-label">{{ t('login.password') }}</label>
            <el-input
              v-model="loginForm.password"
              type="password"
              :placeholder="t('login.passwordPlaceholder')"
              maxlength="20"
              tabindex="2"
              autocomplete="on"
              show-password
              @keyup.enter="handleLogin"
            />
          </el-form-item>
        </div>
        <div class="flex-bar">
          <el-checkbox v-model="loginForm.remember">{{ t('login.remember') }}</el-checkbox>
          <el-link type="primary" underline="never" @click="formType = 'reset'">
            {{ t('login.forgotPassword') }}
          </el-link>
        </div>
        <el-button
          v-blur
          :loading="loading"
          type="primary"
          size="large"
          style="width: 100%"
          @click.prevent="handleLogin"
        >
          {{ t('login.submit') }}
        </el-button>
        <div class="sub-link">
          <span class="text">{{ t('login.noAccount') }}</span>
          <el-link type="primary" underline="never" @click="formType = 'register'">
            {{ t('login.createAccount') }}
          </el-link>
        </div>
        <div class="demo-area">
          <el-divider>{{ t('login.demoAccount') }}</el-divider>
          <button v-blur type="button" class="demo-btn" @click="testAccount('admin')">
            admin <span class="mono">超级管理员</span>
          </button>
        </div>
      </el-form>
      <el-form
        v-show="formType === 'register'"
        ref="registerFormRef"
        :model="registerForm"
        :rules="registerRules"
        class="login-form"
        auto-complete="on"
      >
        <div class="title-container">
          <h3 class="title">{{ t('login.registerTitle') }}</h3>
        </div>
        <div>
          <el-form-item prop="username">
            <el-input
              v-model="registerForm.username"
              :placeholder="t('login.username')"
              maxlength="20"
              tabindex="1"
              autocomplete="on"
            >
              <template #prefix>
                <el-icon>
                  <svg-icon name="ep:user" />
                </el-icon>
              </template>
            </el-input>
          </el-form-item>
          <el-form-item prop="captcha">
            <el-input
              v-model="registerForm.captcha"
              :placeholder="t('login.captcha')"
              tabindex="2"
              autocomplete="on"
            >
              <template #prefix>
                <el-icon>
                  <svg-icon name="ep:key" />
                </el-icon>
              </template>
              <template #append>
                <el-button v-blur>{{ $t('login.sendCaptcha') }}</el-button>
              </template>
            </el-input>
          </el-form-item>
          <el-form-item prop="password">
            <el-input
              v-model="registerForm.password"
              type="password"
              :placeholder="$t('login.password')"
              maxlength="20"
              tabindex="3"
              autocomplete="on"
              show-password
            >
              <template #prefix>
                <el-icon>
                  <svg-icon name="ep:lock" />
                </el-icon>
              </template>
            </el-input>
          </el-form-item>
          <el-form-item prop="checkPassword">
            <el-input
              v-model="registerForm.checkPassword"
              type="password"
              :placeholder="$t('login.confirmPassword')"
              tabindex="4"
              autocomplete="on"
              show-password
            >
              <template #prefix>
                <el-icon>
                  <svg-icon name="ep:lock" />
                </el-icon>
              </template>
            </el-input>
          </el-form-item>
        </div>
        <el-button
          v-blur
          :loading="loading"
          type="primary"
          size="large"
          style="width: 100%; margin-top: 20px"
          @click.prevent="handleRegister"
        >
          {{ $t('login.register') }}
        </el-button>
        <div class="sub-link">
          <span class="text">{{ $t('login.hasAccount') }}</span>
          <el-link type="primary" underline="never" @click="formType = 'login'">{{ $t('login.goLogin') }}</el-link>
        </div>
      </el-form>
      <el-form
        v-show="formType === 'reset'"
        ref="resetFormRef"
        :model="resetForm"
        :rules="resetRules"
        class="login-form"
        auto-complete="on"
      >
        <div class="title-container">
          <h3 class="title">{{ $t('login.resetTitle') }}</h3>
        </div>
        <div>
          <el-form-item prop="username">
            <el-input
              v-model="resetForm.username"
              :placeholder="$t('login.username')"
              tabindex="1"
              autocomplete="on"
              maxlength="20"
            >
              <template #prefix>
                <el-icon>
                  <svg-icon name="ep:user" />
                </el-icon>
              </template>
            </el-input>
          </el-form-item>
          <el-form-item prop="captcha">
            <el-input
              v-model="resetForm.captcha"
              :placeholder="$t('login.captcha')"
              tabindex="2"
              autocomplete="on"
            >
              <template #prefix>
                <el-icon>
                  <svg-icon name="ep:key" />
                </el-icon>
              </template>
              <template #append>
                <el-button v-blur>{{ $t('login.sendCaptcha') }}</el-button>
              </template>
            </el-input>
          </el-form-item>
          <el-form-item prop="newPassword">
            <el-input
              v-model="resetForm.newPassword"
              type="password"
              :placeholder="$t('login.newPassword')"
              tabindex="3"
              autocomplete="on"
              show-password
              maxlength="20"
            >
              <template #prefix>
                <el-icon>
                  <svg-icon name="ep:lock" />
                </el-icon>
              </template>
            </el-input>
          </el-form-item>
        </div>
        <el-button
          v-blur
          :loading="loading"
          type="primary"
          size="large"
          style="width: 100%; margin-top: 20px"
          @click.prevent="handleReset"
        >
          {{ $t('common.confirm') }}
        </el-button>
        <div class="sub-link">
          <el-link type="primary" underline="never" @click="formType = 'login'">
            {{ $t('login.backLogin') }}
          </el-link>
        </div>
      </el-form>
      <p class="login-copyright mono">© 2026 EgoAdmin · MIT License</p>
    </main>
  </div>
</template>

<style lang="scss" scoped>
/* 深色契约面板局部令牌（固定深色，不随主题） */
.login-page {
  --d-bg: #0e1116;
  --d-surface: #161b22;
  --d-border: #222934;
  --d-text: #c2c9d4;
  --d-muted: #8b95a4;
  --d-cyan: #43c6e8;
  --d-accent: #4b9cf5;

  display: grid;
  grid-template-columns: 1.08fr 0.92fr;
  min-height: 100vh;
}

/* ---------- 左：契约面板 ---------- */
.brand-panel {
  position: relative;
  display: flex;
  flex-direction: column;
  padding: 48px 56px;
  overflow: hidden;
  color: var(--d-text);
  background: var(--d-bg);

  .grid-bg {
    position: absolute;
    inset: 0;
    background-image:
      linear-gradient(#1b2230 1px, transparent 1px),
      linear-gradient(90deg, #1b2230 1px, transparent 1px);
    background-size: 36px 36px;
    opacity: 0.5;
    mask-image: radial-gradient(120% 80% at 30% 10%, #000, transparent 72%);
    -webkit-mask-image: radial-gradient(120% 80% at 30% 10%, #000, transparent 72%);
  }

  > * {
    position: relative;
    z-index: 1;
  }

  .bp-logo {
    display: flex;
    align-items: center;
    gap: 11px;
    font-family: var(--font-display);
    font-size: 18px;
    font-weight: 700;
    color: #fff;
    letter-spacing: -0.01em;

    small {
      font-size: 11px;
      font-weight: 500;
      color: var(--d-muted);
    }
  }

  .bp-mid {
    max-width: 480px;
    margin-top: auto;
    margin-bottom: auto;
    padding: 40px 0;

    .bp-eyebrow {
      font-size: 12px;
      font-weight: 600;
      letter-spacing: 0.09em;
      text-transform: uppercase;
      color: var(--d-cyan);
    }

    h2 {
      margin: 14px 0;
      font-size: 32px;
      font-weight: 680;
      line-height: 1.18;
      color: #fff;
      letter-spacing: -0.02em;
    }

    p {
      margin: 0 0 26px;
      font-size: 15px;
      line-height: 1.65;
      color: var(--d-muted);
    }
  }

  .bp-code {
    overflow: hidden;
    background: var(--d-surface);
    border: 1px solid var(--d-border);
    border-radius: var(--r-lg);

    .bar {
      display: flex;
      align-items: center;
      gap: 7px;
      padding: 11px 14px;
      border-bottom: 1px solid var(--d-border);
    }

    .dot {
      width: 10px;
      height: 10px;
      border-radius: 50%;
    }

    .file {
      margin-left: 6px;
      font-size: 12px;
      color: var(--d-muted);
    }

    pre {
      margin: 0;
      padding: 16px;
      overflow: auto;
      font-size: 12.5px;
      line-height: 1.7;
      color: var(--d-text);
    }

    .k { color: var(--d-accent); font-weight: 600; }
    .s { color: #7dd491; }
    .c { color: #5a6573; }
    .t { color: var(--d-cyan); }
  }

  .bp-chips {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
    margin-top: 22px;

    span {
      padding: 5px 11px;
      font-size: 12px;
      color: var(--d-text);
      background: #1b2230;
      border: 1px solid var(--d-border);
      border-radius: var(--r-pill);
    }
  }

  .bp-foot {
    font-size: 11.5px;
    color: var(--d-muted);
  }
}

/* ---------- 右：登录卡 ---------- */
.form-panel {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px;
  background: var(--bg);

  .login-form {
    width: 100%;
    max-width: 380px;

    .title-container {
      margin-bottom: 30px;

      .title {
        margin: 0 0 8px;
        font-family: var(--font-display);
        font-size: 26px;
        font-weight: 660;
        color: var(--fg);
        letter-spacing: -0.012em;
      }

      .sub {
        margin: 0;
        font-size: 14px;
        color: var(--muted);
      }
    }
  }

  .el-form-item {
    margin-bottom: 18px;

    &.labeled-field {
      display: block;

      .field-label {
        display: block;
        margin-bottom: 7px;
        font-size: 13px;
        font-weight: 550;
        color: var(--text);
        line-height: 1.4;
      }

      :deep(.el-form-item__content) {
        line-height: normal;
      }
    }

    :deep(.el-input) {
      width: 100%;
      height: 44px;

      .el-input__wrapper {
        border-radius: var(--r-md);
      }

      input {
        height: 44px;
      }
    }
  }

  .flex-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 22px;
  }

  .sub-link {
    display: flex;
    align-items: center;
    justify-content: center;
    margin-top: 24px;
    font-size: 13.5px;
    color: var(--muted);

    .text {
      margin-right: 10px;
    }
  }

  .demo-area {
    margin-top: 26px;

    :deep(.el-divider__text) {
      font-size: 12.5px;
      color: var(--faint);
      background: var(--bg);
    }
  }

  .demo-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 7px;
    width: 100%;
    height: 38px;
    margin-top: 4px;
    font: inherit;
    font-size: 13px;
    font-weight: 550;
    color: var(--text);
    cursor: pointer;
    background: var(--surface);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-md);
    transition: border-color 0.15s var(--ease), color 0.15s var(--ease), background 0.15s var(--ease);

    .mono {
      font-size: 12px;
      color: var(--muted);
    }

    &:hover {
      color: var(--accent-text);
      background: var(--accent-weak);
      border-color: var(--accent);

      .mono {
        color: var(--accent-text);
      }
    }
  }

  :deep(.el-button--large) {
    height: 44px;
    font-weight: 600;
  }

  .login-copyright {
    position: absolute;
    bottom: 24px;
    left: 0;
    width: 100%;
    margin: 0;
    font-size: 12px;
    color: var(--faint);
    text-align: center;
  }
}

:deep(input[type='password']::-ms-reveal) {
  display: none;
}

/* 移动端：隐藏左面板，登录卡占满 */
@media screen and (max-width: 880px) {
  .login-page {
    grid-template-columns: 1fr;
  }

  .brand-panel {
    display: none;
  }
}

[data-mode='mobile'] {
  .login-page {
    grid-template-columns: 1fr;
  }

  .brand-panel {
    display: none;
  }

  .form-panel {
    padding: 32px 24px;
  }
}
</style>
