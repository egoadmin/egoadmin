import { defineConfig } from 'vitepress'

const base = process.env.DOCS_BASE || '/'
const withBase = (path: string) => `${base}${path.replace(/^\//, '')}`

const zhGuide = [
  {
    text: '入门指南',
    collapsed: false,
    items: [
      { text: '快速开始', link: '/guide/getting-started' },
    ],
  },
  {
    text: '核心功能总览',
    collapsed: false,
    items: [
      { text: '总览', link: '/guide/features' },
    ],
  },
  {
    text: '架构与设计',
    collapsed: false,
    items: [
      { text: '微服务架构', link: '/guide/architecture' },
      { text: 'API 开发工作流', link: '/guide/api-development' },
      { text: '权限系统', link: '/guide/permission-system' },
      { text: '数据库与迁移', link: '/guide/database-migration' },
      { text: '前端开发', link: '/guide/frontend-development' },
      { text: '设计资源', link: '/guide/design-resources' },
      { text: '分布式事务（DTM）', link: '/guide/distributed-transactions' },
      { text: '组件系统', link: '/guide/components' },
      { text: '运行时配置', link: '/guide/configuration' },
      { text: '测试与部署', link: '/guide/testing-deployment' },
    ],
  },
]

const enGuide = [
  {
    text: 'Getting Started',
    collapsed: false,
    items: [
      { text: 'Getting Started', link: '/en-US/guide/getting-started' },
    ],
  },
  {
    text: 'Feature Overview',
    collapsed: false,
    items: [
      { text: 'Overview', link: '/en-US/guide/features' },
    ],
  },
  {
    text: 'Architecture & Design',
    collapsed: false,
    items: [
      { text: 'Microservice Architecture', link: '/en-US/guide/architecture' },
      { text: 'API Development Workflow', link: '/en-US/guide/api-development' },
      { text: 'Permission System', link: '/en-US/guide/permission-system' },
      { text: 'Database & Migrations', link: '/en-US/guide/database-migration' },
      { text: 'Frontend Development', link: '/en-US/guide/frontend-development' },
      { text: 'Design Assets', link: '/en-US/guide/design-resources' },
      { text: 'Distributed Transactions (DTM)', link: '/en-US/guide/distributed-transactions' },
      { text: 'Component System', link: '/en-US/guide/components' },
      { text: 'Runtime Configuration', link: '/en-US/guide/configuration' },
      { text: 'Testing & Deployment', link: '/en-US/guide/testing-deployment' },
    ],
  },
]

export default defineConfig({
  title: 'EgoAdmin',
  description: '基于 EGO 框架的 Go 微服务后台管理模板',
  base,
  lang: 'zh-CN',
  lastUpdated: true,
  cleanUrls: true,
  ignoreDeadLinks: true,

  rewrites: {
    'guide/zh-CN/:page': 'guide/:page',
    'guide/en-US/:page': 'en-US/guide/:page',
  },

  markdown: {
    lineNumbers: true,
    container: {
      tipLabel: '提示',
      warningLabel: '警告',
      dangerLabel: '危险',
    },
  },

  locales: {
    root: {
      label: '简体中文',
      lang: 'zh-CN',
      title: 'EgoAdmin',
      description: '基于 EGO 框架的 Go 微服务后台管理模板',
      themeConfig: {
        nav: [
          { text: '首页', link: '/' },
          { text: '快速开始', link: '/guide/getting-started' },
          { text: '核心功能', link: '/guide/features' },
          { text: '架构', link: '/guide/architecture' },
          { text: '设计资源', link: '/design/' },
          { text: 'GitHub', link: 'https://github.com/egoadmin/egoadmin' },
        ],
        sidebar: {
          '/guide/': zhGuide,
        },
        outline: {
          level: [2, 3],
          label: '本页目录',
        },
        lastUpdated: {
          text: '最后更新于',
          formatOptions: {
            dateStyle: 'short',
            timeStyle: 'short',
          },
        },
        docFooter: {
          prev: '上一页',
          next: '下一页',
        },
        editLink: {
          pattern: 'https://github.com/egoadmin/egoadmin/edit/main/docs/:path',
          text: '在 GitHub 上编辑此页',
        },
      },
    },
    'en-US': {
      label: 'English',
      lang: 'en-US',
      link: '/en-US/',
      title: 'EgoAdmin',
      description: 'A Go microservice admin template based on EGO',
      themeConfig: {
        nav: [
          { text: 'Home', link: '/en-US/' },
          { text: 'Getting Started', link: '/en-US/guide/getting-started' },
          { text: 'Features', link: '/en-US/guide/features' },
          { text: 'Architecture', link: '/en-US/guide/architecture' },
          { text: 'Design Assets', link: '/design/' },
          { text: 'GitHub', link: 'https://github.com/egoadmin/egoadmin' },
        ],
        sidebar: {
          '/en-US/guide/': enGuide,
        },
        outline: {
          level: [2, 3],
          label: 'On this page',
        },
        lastUpdated: {
          text: 'Last updated',
          formatOptions: {
            dateStyle: 'short',
            timeStyle: 'short',
          },
        },
        docFooter: {
          prev: 'Previous page',
          next: 'Next page',
        },
        editLink: {
          pattern: 'https://github.com/egoadmin/egoadmin/edit/main/docs/:path',
          text: 'Edit this page on GitHub',
        },
      },
    },
  },

  themeConfig: {
    siteTitle: 'EgoAdmin',
    logo: '/logo.svg',
    search: {
      provider: 'local',
      options: {
        locales: {
          root: {
            translations: {
              button: {
                buttonText: '搜索',
                buttonAriaLabel: '搜索',
              },
              modal: {
                noResultsText: '没有找到结果',
                resetButtonTitle: '清除查询',
                footer: {
                  selectText: '选择',
                  navigateText: '切换',
                  closeText: '关闭',
                },
              },
            },
          },
          'en-US': {
            translations: {
              button: {
                buttonText: 'Search',
                buttonAriaLabel: 'Search',
              },
              modal: {
                noResultsText: 'No results found',
                resetButtonTitle: 'Clear query',
                footer: {
                  selectText: 'to select',
                  navigateText: 'to navigate',
                  closeText: 'to close',
                },
              },
            },
          },
        },
      },
    },
    socialLinks: [
      { icon: 'github', link: 'https://github.com/egoadmin/egoadmin' },
    ],
    footer: {
      message: 'Released under the MIT License.',
      copyright: 'Copyright © 2024 EgoAdmin Contributors',
    },
  },

  head: [
    ['link', { rel: 'icon', href: withBase('/favicon.svg') }],
    ['meta', { name: 'theme-color', content: '#3b82f6' }],
  ],
})
