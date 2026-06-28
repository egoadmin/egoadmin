import ElementPlus from 'element-plus'
import VueCropper from 'vue-cropper'
import App from './App.vue'
import pinia from './store'
import router from './router'
import useSettingsStore from './store/modules/settings'
import i18n from './i18n'
import 'vue-cropper/dist/index.css'
import { toCustomDate } from './utils/globalFun'

// 自定义指令
import directive from '@/utils/directive'

// 加载 svg 图标
import 'virtual:svg-icons-register'

// 全局样式
import '@/styles/globals.scss'

// 加载 iconify 图标（element plus）
import { downloadAndInstall } from '@/iconify-ep'

const app = createApp(App)
app.use(ElementPlus)
app.use(pinia)
app.use(i18n)
app.use(router).use(VueCropper)
directive(app)
if (useSettingsStore().settings.app.iconifyOfflineUse) {
  downloadAndInstall()
}
app.config.globalProperties.$toCustomDate = toCustomDate

app.mount('#app')
