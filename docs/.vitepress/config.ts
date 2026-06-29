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
      { text: '后端开发详解', link: '/guide/backend-dev' },
      { text: '权限系统', link: '/guide/permission-system' },
      {
        text: '数据库与迁移',
        items: [
          { text: '概览', link: '/guide/database-migration' },
          { text: 'GORM 模型与仓储', link: '/guide/database/gorm-models' },
          { text: '查询与事务', link: '/guide/database/gorm-queries' },
          { text: 'Atlas 迁移管理', link: '/guide/database/atlas-migration' },
        ],
      },
      {
        text: '前端开发',
        items: [
          { text: '概览', link: '/guide/frontend-development' },
          { text: '前端路由与权限', link: '/guide/frontend/routing' },
          { text: '状态管理', link: '/guide/frontend/state-management' },
          { text: '主题与设计系统', link: '/guide/frontend/theme' },
        ],
      },
      { text: '设计资源', link: '/guide/design-resources' },
      { text: '分布式事务（DTM）', link: '/guide/distributed-transactions' },
      {
        text: '组件系统',
        items: [
          { text: '概览', link: '/guide/components' },
          { text: 'AuthSession 认证会话', link: '/guide/components/auth-session' },
          { text: 'LoginCrypto 登录加密', link: '/guide/components/login-crypto' },
          { text: 'IDGen ID 生成', link: '/guide/components/idgen' },
          { text: 'ERedis 缓存', link: '/guide/components/eredis' },
        ],
      },
      { text: '运行时配置', link: '/guide/configuration' },
      { text: '优雅停机与生命周期', link: '/guide/graceful-shutdown' },
      { text: '国际化（i18n）', link: '/guide/i18n' },
      { text: 'Job 与定时任务', link: '/guide/job-cron' },
      {
        text: '部署与运维',
        items: [
          { text: '测试与部署', link: '/guide/testing-deployment' },
          { text: 'Docker 容器化', link: '/guide/deployment/docker' },
          { text: 'CI/CD 流水线', link: '/guide/deployment/cicd' },
          { text: '监控与告警', link: '/guide/deployment/monitoring' },
          { text: '性能优化', link: '/guide/deployment/performance' },
        ],
      },
    ],
  },
  {
    text: '编码规范',
    collapsed: false,
    items: [
      { text: 'Go 语言编码规范', link: '/guide/standards/go' },
      { text: 'Vue3 前端开发规范', link: '/guide/standards/vue3' },
      { text: 'API 设计规范', link: '/guide/standards/api' },
    ],
  },
  {
    text: '排错指南',
    collapsed: false,
    items: [
      { text: '常见问题诊断', link: '/guide/troubleshooting/common-issues' },
      { text: '调试工具使用', link: '/guide/troubleshooting/debug-tools' },
      { text: '性能问题排查', link: '/guide/troubleshooting/performance' },
      { text: '配置问题诊断', link: '/guide/troubleshooting/config-issues' },
      { text: '日志分析', link: '/guide/troubleshooting/log-analysis' },
    ],
  },
  {
    text: '安全加固',
    collapsed: false,
    items: [
      { text: '认证与会话安全', link: '/guide/security/auth-security' },
      { text: '密码安全', link: '/guide/security/password-security' },
      { text: '攻击防护', link: '/guide/security/attack-protection' },
      { text: '安全审计', link: '/guide/security/audit' },
    ],
  },
  {
    text: '第三方集成',
    collapsed: false,
    items: [
      { text: '对象存储（MinIO）', link: '/guide/third-party/minio' },
      { text: '搜索服务（MeiliSearch）', link: '/guide/third-party/meilisearch' },
      { text: '消息通知', link: '/guide/third-party/messaging' },
      { text: '支付集成', link: '/guide/third-party/payment' },
    ],
  },
  {
    text: '微服务详解',
    collapsed: false,
    items: [
      {
        text: '网关服务（Gateway）',
        items: [
          { text: 'API 路由与转发', link: '/guide/gateway/api-routing' },
          { text: '鉴权与权限控制', link: '/guide/gateway/auth-permission' },
          { text: '文件上传与 Web 服务', link: '/guide/gateway/upload-web' },
          { text: '监控与运维', link: '/guide/gateway/monitoring' },
        ],
      },
      {
        text: '用户服务（User）',
        items: [
          { text: '用户与部门管理', link: '/guide/user-service/user-dept' },
          { text: '角色与权限', link: '/guide/user-service/role-permission' },
          { text: '数据权限（DataScope）', link: '/guide/user-service/data-permission' },
          { text: '审计日志', link: '/guide/user-service/audit-log' },
        ],
      },
      {
        text: 'ID 生成服务（IdGen）',
        items: [
          { text: '号段与雪花算法', link: '/guide/idgen/segment-snowflake' },
          { text: 'gRPC 接口设计', link: '/guide/idgen/grpc-api' },
          { text: '机器租约管理', link: '/guide/idgen/machine-lease' },
        ],
      },
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
      { text: 'Backend Development Guide', link: '/en-US/guide/backend-dev' },
      { text: 'Permission System', link: '/en-US/guide/permission-system' },
      {
        text: 'Database & Migrations',
        items: [
          { text: 'Overview', link: '/en-US/guide/database-migration' },
          { text: 'GORM Models & Repositories', link: '/en-US/guide/database/gorm-models' },
          { text: 'Queries & Transactions', link: '/en-US/guide/database/gorm-queries' },
          { text: 'Atlas Migration', link: '/en-US/guide/database/atlas-migration' },
        ],
      },
      {
        text: 'Frontend Development',
        items: [
          { text: 'Overview', link: '/en-US/guide/frontend-development' },
          { text: 'Routing & Permissions', link: '/en-US/guide/frontend/routing' },
          { text: 'State Management', link: '/en-US/guide/frontend/state-management' },
          { text: 'Theme & Design System', link: '/en-US/guide/frontend/theme' },
        ],
      },
      { text: 'Design Assets', link: '/en-US/guide/design-resources' },
      { text: 'Distributed Transactions (DTM)', link: '/en-US/guide/distributed-transactions' },
      {
        text: 'Component System',
        items: [
          { text: 'Overview', link: '/en-US/guide/components' },
          { text: 'AuthSession', link: '/en-US/guide/components/auth-session' },
          { text: 'LoginCrypto', link: '/en-US/guide/components/login-crypto' },
          { text: 'IDGen', link: '/en-US/guide/components/idgen' },
          { text: 'ERedis', link: '/en-US/guide/components/eredis' },
        ],
      },
      { text: 'Runtime Configuration', link: '/en-US/guide/configuration' },
      { text: 'Graceful Shutdown & Lifecycle', link: '/en-US/guide/graceful-shutdown' },
      { text: 'Internationalization (i18n)', link: '/en-US/guide/i18n' },
      { text: 'Job & Cron Tasks', link: '/en-US/guide/job-cron' },
      {
        text: 'Deployment & Ops',
        items: [
          { text: 'Testing & Deployment', link: '/en-US/guide/testing-deployment' },
          { text: 'Docker', link: '/en-US/guide/deployment/docker' },
          { text: 'CI/CD Pipeline', link: '/en-US/guide/deployment/cicd' },
          { text: 'Monitoring & Alerting', link: '/en-US/guide/deployment/monitoring' },
          { text: 'Performance Tuning', link: '/en-US/guide/deployment/performance' },
        ],
      },
    ],
  },
  {
    text: 'Coding Standards',
    collapsed: false,
    items: [
      { text: 'Go Coding Standards', link: '/en-US/guide/standards/go' },
      { text: 'Vue3 Frontend Standards', link: '/en-US/guide/standards/vue3' },
      { text: 'API Design Standards', link: '/en-US/guide/standards/api' },
    ],
  },
  {
    text: 'Troubleshooting',
    collapsed: false,
    items: [
      { text: 'Common Issues', link: '/en-US/guide/troubleshooting/common-issues' },
      { text: 'Debug Tools', link: '/en-US/guide/troubleshooting/debug-tools' },
      { text: 'Performance Issues', link: '/en-US/guide/troubleshooting/performance' },
      { text: 'Config Diagnostics', link: '/en-US/guide/troubleshooting/config-issues' },
      { text: 'Log Analysis', link: '/en-US/guide/troubleshooting/log-analysis' },
    ],
  },
  {
    text: 'Security Hardening',
    collapsed: false,
    items: [
      { text: 'Auth & Session Security', link: '/en-US/guide/security/auth-security' },
      { text: 'Password Security', link: '/en-US/guide/security/password-security' },
      { text: 'Attack Protection', link: '/en-US/guide/security/attack-protection' },
      { text: 'Security Audit', link: '/en-US/guide/security/audit' },
    ],
  },
  {
    text: 'Third-Party Integrations',
    collapsed: false,
    items: [
      { text: 'Object Storage (MinIO)', link: '/en-US/guide/third-party/minio' },
      { text: 'Search (MeiliSearch)', link: '/en-US/guide/third-party/meilisearch' },
      { text: 'Messaging & Notifications', link: '/en-US/guide/third-party/messaging' },
      { text: 'Payment Integration', link: '/en-US/guide/third-party/payment' },
    ],
  },
  {
    text: 'Microservice Deep Dive',
    collapsed: false,
    items: [
      {
        text: 'Gateway Service',
        items: [
          { text: 'API Routing & Forwarding', link: '/en-US/guide/gateway/api-routing' },
          { text: 'Auth & Permission', link: '/en-US/guide/gateway/auth-permission' },
          { text: 'File Upload & Web', link: '/en-US/guide/gateway/upload-web' },
          { text: 'Monitoring & Ops', link: '/en-US/guide/gateway/monitoring' },
        ],
      },
      {
        text: 'User Service',
        items: [
          { text: 'User & Department', link: '/en-US/guide/user-service/user-dept' },
          { text: 'Role & Permission', link: '/en-US/guide/user-service/role-permission' },
          { text: 'Data Permission', link: '/en-US/guide/user-service/data-permission' },
          { text: 'Audit Log', link: '/en-US/guide/user-service/audit-log' },
        ],
      },
      {
        text: 'ID Generation Service',
        items: [
          { text: 'Segment & Snowflake', link: '/en-US/guide/idgen/segment-snowflake' },
          { text: 'gRPC API Design', link: '/en-US/guide/idgen/grpc-api' },
          { text: 'Machine Lease', link: '/en-US/guide/idgen/machine-lease' },
        ],
      },
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
    'guide/zh-CN/:page(.*)': 'guide/:page',
    'guide/en-US/:page(.*)': 'en-US/guide/:page',
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
          { text: '设计资源', link: '/design/', target: '_blank', rel: 'noopener noreferrer' },
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
          { text: 'Design Assets', link: '/design/', target: '_blank', rel: 'noopener noreferrer' },
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
