// 该文件为系统默认配置，请勿修改！！！

import type { RecursiveRequired, Settings } from '#/global'

const globalSettingsDefault: RecursiveRequired<Settings.all> = {
  app: {
    colorScheme: 'light',
    elementSize: 'default',
    iconifyOfflineUse: false,
    enablePermission: false,
    enableProgress: true,
    enableDynamicTitle: false,
    routeBaseOn: 'frontend',
    enableAppSetting: false,
    tabbarDirection: 'horizontal',
  },
  home: {
    enable: true,
    title: '主页',
  },
  layout: {
    enableMobileAdaptation: false,
    showNavDirSwitch: true,
  },
  menu: {
    baseOn: 'frontend',
    menuMode: 'head',
    switchMainMenuAndPageJump: false,
    subMenuUniqueOpened: true,
    subMenuCollapse: false,
    enableSubMenuCollapseButton: true,
    enableHotkeys: false,
  },
  topbar: {
    mode: 'static',
  },
  toolbar: {
    enableFullscreen: false,
    enablePageReload: false,
    enableColorScheme: false,
  },
  breadcrumb: {
    enable: true,
  },
  navSearch: {
    enable: false,
    enableHotkeys: false,
  },
  copyright: {
    enable: true,
    dates: '2020-2023',
    company: 'Fantastic-admin',
    website: 'https://hooray.gitee.io/fantastic-admin/',
    beian: '',
  },
}

export default globalSettingsDefault
