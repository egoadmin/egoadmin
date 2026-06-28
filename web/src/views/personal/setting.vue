<route lang="yaml">
name: personalSetting
meta:
  title: menu.personalSetting
  cache: personal-edit.password
</route>

<script lang="ts" setup name="PersonalSetting">
import { ElMessage } from 'element-plus'
import Fingerprint2 from 'fingerprintjs2'
import SettingsPass from './components/settings-pass.vue'
import SettingsImg from './components/settings-img.vue'
import SettingsPhone from './components/settings-phone.vue'
import useUserStore from '@/store/modules/user'
import apiUser from '@/api/modules/user'
import { LOGIN_CRYPTO_ACTION, encryptPasswordPayload } from '@/utils/login-crypto'
import { avatarUrl } from '@/utils'
import { t } from '@/i18n'

interface Person {
  nameShow: string
  genderShow: string
}
type PersonProps = keyof Person

const userStore = useUserStore()
const userInfo = computed(() => userStore.userInfo)
const currentAvatarUrl = computed(() => avatarUrl(userInfo.value))
const ua = ref('')
function createFingerprint() {
  return new Promise<string>((resolve) => {
    Fingerprint2.get((components: any) => {
      const values = components.map((component: any) => component.value)
      ua.value = Fingerprint2.x64hash128(values.join(''), 31)
      resolve(ua.value)
    })
  })
}
createFingerprint()
const settingPassRef = ref()
//  打开修改密码弹窗
function openPassDialog() {
  settingPassRef.value.dialogFormVisibleChange(true)
}

const settingsImgRef = ref()
//  打开头像弹窗
function openImgDialog() {
  settingsImgRef.value.dialogImgVisibleChange(true)
}

const settingsPhoneRef = ref()
//  打开手机号修改弹窗
function openPhoneDialog() {
  settingsPhoneRef.value.dialogPhoneVisibleChange(true)
}

const updateItem = ref({
  name: '',
  nameErr: '',
  nameShow: false,
  gender: 0,
  genderShow: false,
})

// 获取性别
function getGender(gender: number) {
  if (!gender) {
    return t('common.secret')
  }
  if (gender === 1) {
    return t('common.male')
  }
  if (gender === 2) {
    return t('common.female')
  }
}

// 获取手机号脱敏
function getPhone(phone: string) {
  if (!phone) {
    return '—'
  }
  return phone.replace(/(\d{3})\d{4}(\d{4})/, '$1****$2')
}

// 把用户信息赋值给编辑项
function updateItemAssign() {
  const { name, gender } = userInfo.value
  updateItem.value.name = name
  updateItem.value.gender = gender
}

// 编辑显示控制
function updateItemShow(key?: PersonProps, val?: boolean) {
  updateItem.value.nameShow = false
  updateItem.value.genderShow = false
  updateItem.value.nameErr = ''
  if (key && val) {
    updateItem.value[key] = val
  }
}

// 名字校验
function verifyName() {
  const reg = /^[\u4E00-\u9FA5a-zA-Z]{2,20}$/
  updateItem.value.nameErr = reg.test(updateItem.value.name)
    ? ''
    : t('personal.nameInvalid')
  if (!updateItem.value.name) {
    updateItem.value.nameErr = t('personal.nameRequired')
  }
}

// 打开编辑项
function openUpdateItem(key: PersonProps) {
  updateItemAssign()
  updateItemShow(key, true)
}

// 关闭编辑项
function closeUpdateItem(key: PersonProps) {
  updateItemShow(key, false)
}
// 修改密码
async function editPassword(params: any) {
  const { oldPassword, password } = params
  const data = await encryptPasswordPayload({
    username: userStore.username,
    ua: ua.value || (await createFingerprint()),
    action: LOGIN_CRYPTO_ACTION.centerEditPassword,
    oldPassword,
    newPassword: password,
  })
  await apiUser.passwordEdit(data)
  settingPassRef.value.dialogFormVisibleChange(false)
  ElMessage.success(t('personal.passwordEditSuccess'))
  setTimeout(() => {
    userStore.logout()
  }, 3000)
}
// 修改用户信息
async function editInfo(data = {}, message = t('personal.phoneEditSuccess'), type?: 'name' | 'gender' | 'phone') {
  if (type === 'name' && updateItem.value.nameErr) {
    return
  }
  const rawInfo: any = { ...updateItem.value, ...data }
  let info = rawInfo
  if (rawInfo.phone && rawInfo.password) {
    const encrypted = await encryptPasswordPayload({
      username: userStore.username,
      ua: ua.value || (await createFingerprint()),
      action: LOGIN_CRYPTO_ACTION.centerEditInfo,
      password: rawInfo.password,
    })
    const { password, ...rest } = rawInfo
    info = { ...rest, ...encrypted }
  }
  console.log(updateItem.value)
  await apiUser.editCenterInfo(info)
  settingsPhoneRef.value.dialogPhoneVisibleChange(false)
  ElMessage.success(message)
  userStore.getUserInfo()
  updateItemShow()
}
// 修改头像
async function editAvatar(data: any) {
  await apiUser.editCenterAvatar(data)
  ElMessage.success(t('personal.avatarEditSuccess'))
  userStore.getUserInfo()
  settingsImgRef.value.dialogImgVisibleChange(false)
}
</script>

<template>
  <div>
    <SettingsPass ref="settingPassRef" @edit-password="editPassword" />
    <SettingsImg ref="settingsImgRef" @edit-avatar="editAvatar" />
    <SettingsPhone ref="settingsPhoneRef" @edit-info="editInfo" />
    <page-main>
      <div class="profile">
        <h3>{{ t('personal.title') }}</h3>
        <div class="avatar-block">
          <div class="head-portrait" @click="openImgDialog">
            <img v-if="currentAvatarUrl" :src="currentAvatarUrl" alt="" />
            <span v-else class="ph-init">{{ (userInfo.name || userInfo.username || 'U').charAt(0).toUpperCase() }}</span>
            <div class="mask">
              <el-icon><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M3 7h4l2-2h6l2 2h4v12H3z" stroke-linejoin="round" /><circle cx="12" cy="13" r="3.5" /></svg></el-icon>
              {{ t('personal.changeAvatar') }}
            </div>
          </div>
        </div>
        <section>
          <div class="title">{{ t('personal.personalInfo') }} <span class="line" /></div>
          <ul>
            <li>
              <div class="label">{{ t('personal.username') }}</div>
              <div class="val mono">
                {{ userInfo.username ? userInfo.username?.replace(/\s/g, '&nbsp;') : '—' }}
              </div>
            </li>
            <li>
              <div class="label">{{ t('personal.name') }}</div>

              <el-form-item v-if="updateItem.nameShow" :error="updateItem.nameErr" class="inline-edit">
                <el-input
                  v-model="updateItem.name"
                  :placeholder="t('personal.namePlaceholder')"
                  style="width: 234px"
                  clearable
                  maxlength="20"
                  @blur="verifyName"
                />
              </el-form-item>
              <div v-else class="val">
                {{ userInfo.name ? userInfo.name?.replace(/\s/g, '&nbsp;') : '—' }}
              </div>
              <div v-if="updateItem.nameShow" class="edit-link">
                <span @click="editInfo({}, t('personal.nameEditSuccess'), 'name')">{{ t('common.save') }}</span>
                <span @click="closeUpdateItem('nameShow')">{{ t('common.cancel') }}</span>
              </div>
              <div v-else class="edit-link">
                <span @click="openUpdateItem('nameShow')">{{ t('common.edit') }}</span>
              </div>
            </li>
            <li>
              <div class="label">{{ t('personal.gender') }}</div>

              <el-form-item v-if="updateItem.genderShow" class="inline-edit">
                <el-radio-group v-model="updateItem.gender">
                  <el-radio :value="1" size="large">{{ t('common.male') }}</el-radio>
                  <el-radio :value="2" size="large">{{ t('common.female') }}</el-radio>
                </el-radio-group>
              </el-form-item>
              <div v-else class="val">
                {{ getGender(userInfo.gender) }}
              </div>
              <div v-if="updateItem.genderShow" class="edit-link">
                <span @click="editInfo({}, t('personal.genderEditSuccess'), 'gender')">{{ t('common.save') }}</span>
                <span @click="closeUpdateItem('genderShow')">{{ t('common.cancel') }}</span>
              </div>
              <div v-else class="edit-link">
                <span @click="openUpdateItem('genderShow')">{{ t('common.edit') }}</span>
              </div>
            </li>
            <li>
              <div class="label">{{ t('personal.organization') }}</div>
              <div class="val">
                {{ userInfo.deptName || '—' }}
              </div>
            </li>
            <li>
              <div class="label">{{ t('personal.role') }}</div>
              <div class="val">
                {{ userInfo.roleNames ? userInfo.roleNames[0] : '—' }}
              </div>
            </li>
          </ul>
        </section>
        <section>
          <div class="title">{{ t('personal.accountSecurity') }} <span class="line" /></div>
          <ul>
            <li>
              <div class="label">{{ t('personal.loginPassword') }}</div>
              <div class="val mask-pwd">••••••••</div>
              <div class="edit-link" @click="openPassDialog">
                <span>{{ t('personal.changePassword') }}</span>
              </div>
            </li>
            <li>
              <div class="label">{{ t('personal.phone') }}</div>
              <div class="val mono">
                {{ getPhone(userInfo.phone) }}
              </div>
              <div class="edit-link">
                <span @click="openPhoneDialog">{{ t('personal.changePhone') }}</span>
              </div>
            </li>
          </ul>
        </section>
      </div>
    </page-main>
  </div>
</template>

<style lang="scss" scoped>
ul,
li {
  list-style: none;
  padding: 0;
}

// 个人中心内容居中收窄（设计 .page max 980）
.profile {
  max-width: 980px;
  margin: 0 auto;
}

h3 {
  font-family: var(--font-display);
  font-size: 22px;
  font-weight: 620;
  color: var(--fg);
  margin: 0;
  padding: 6px 0 4px 4px;
}

.avatar-block {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  margin: 18px 0 30px;
}

.head-portrait {
  position: relative;
  width: 96px;
  height: 96px;
  border-radius: 50%;
  overflow: hidden;
  cursor: pointer;
  background: var(--accent-weak);

  .ph-init {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 100%;
    height: 100%;
    font-family: var(--font-display);
    font-size: 36px;
    font-weight: 600;
    color: var(--accent-text);
  }

  & > img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .mask {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 5px;
    font-size: 12.5px;
    color: #fff;
    background: rgb(0 0 0 / 50%);
    opacity: 0;
    transition: opacity 0.18s var(--ease);
  }

  &:hover .mask {
    opacity: 1;
  }
}

section {
  display: flex;
  flex-direction: column;
  align-items: center;
  margin-top: 30px;

  .title {
    width: 100%;
    max-width: 760px;
    font-size: 16px;
    font-weight: 600;
    color: var(--fg);
    display: flex;
    align-items: center;
    gap: 12px;
    padding-left: 2px;
    margin-bottom: 4px;

    &::before {
      content: '';
      display: block;
      width: 4px;
      height: 18px;
      border-radius: 2px;
      background: var(--accent);
      flex: none;
    }

    .line {
      flex: 1;
      height: 1px;
      background: var(--border);
    }
  }

  ul {
    width: 100%;
    max-width: 760px;
  }

  li {
    width: 100%;
    display: flex;
    font-size: 14px;
    min-height: 56px;
    padding: 10px 4px;
    gap: 20px;
    align-items: center;
    border-bottom: 1px solid var(--border);

    &:last-child {
      border-bottom: 0;
    }

    & > .el-form-item.inline-edit {
      margin-bottom: 0;
      flex: 1;

      :deep(.el-form-item__error) {
        white-space: nowrap;
      }
    }

    .label {
      width: 100px;
      color: var(--muted);
      font-size: 13.5px;
      flex-shrink: 0;
    }

    .val {
      flex: 1;
      color: var(--fg);

      &.mask-pwd {
        letter-spacing: 2px;
        color: var(--muted);
      }
    }

    .edit-link {
      margin-left: auto;
      display: inline-flex;
      gap: 14px;
      color: var(--accent-text);
      font-size: 13.5px;
      cursor: pointer;

      span:hover {
        color: var(--accent-hover);
      }
    }
  }
}

.el-radio {
  margin-right: 20px;
}
</style>
