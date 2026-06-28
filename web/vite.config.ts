import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath, URL } from 'node:url'
import { defineConfig, loadEnv } from 'vite-plus'
import dayjs from 'dayjs'
import pkg from './package.json' with { type: 'json' }
import createVitePlugins from './vite/plugins/index.ts'

// https://vitejs.dev/config/
const mode = process.env.NODE_ENV || 'production'
const command = process.env.VP_COMMAND || 'build'
const env = loadEnv(mode, process.cwd())

// 全局 scss 资源
const scssResources: string[] = []
fs.readdirSync('src/styles/resources').forEach((dirname) => {
  if (fs.statSync(`src/styles/resources/${dirname}`).isFile()) {
    scssResources.push(`@use "${path.resolve(`src/styles/resources/${dirname}`)}" as *;`)
  }
})
// css 精灵图相关
fs.readdirSync('src/assets/sprites').forEach((dirname) => {
  if (fs.statSync(`src/assets/sprites/${dirname}`).isDirectory()) {
    // css 精灵图生成的 scss 文件也需要放入全局 scss 资源
    scssResources.push(`@use "${path.resolve(`src/assets/sprites/_${dirname}.scss`)}" as *;`)
  }
})

export default defineConfig({
  staged: {
    '*.{ts,tsx,js,jsx,vue}': 'vp check --fix',
  },
  fmt: {
    semi: false,
    singleQuote: true,
    ignorePatterns: [
      '**/dist/**',
      '**/dist-ssr/**',
      '**/node_modules/**',
      'public/**',
      'plop-templates/**',
      '**/*.hbs',
      '**/*.md',
      '**/*.yml',
      '**/*.yaml',
      'src/styles/**',
    ],
  },
  lint: {
    plugins: ['eslint', 'typescript', 'unicorn', 'oxc', 'vue'],
    categories: {
      correctness: 'error',
    },
    env: {
      browser: true,
      builtin: true,
    },
    ignorePatterns: [
      '**/dist/**',
      '**/dist-ssr/**',
      '**/coverage/**',
      'public/**/*.js',
      'plop-templates/**/*',
      '**/*.md',
      '**/*.yml',
      '**/*.yaml',
    ],
    rules: {
      'no-console': 'off',
      'no-var': 'error',
      'prefer-const': 'error',
      curly: ['error', 'all'],
    },
    overrides: [
      {
        files: ['**/*.ts', '**/*.tsx', '**/*.mts', '**/*.cts', '**/*.vue'],
        rules: {
          'constructor-super': 'off',
          'getter-return': 'off',
          'no-class-assign': 'off',
          'no-const-assign': 'off',
          'no-dupe-class-members': 'off',
          'no-dupe-keys': 'off',
          'no-func-assign': 'off',
          'no-import-assign': 'off',
          'no-new-native-nonconstructor': 'off',
          'no-obj-calls': 'off',
          'no-redeclare': 'off',
          'no-setter-return': 'off',
          'no-this-before-super': 'off',
          'no-undef': 'off',
          'no-unreachable': 'off',
          'no-unsafe-negation': 'off',
          'no-with': 'off',
          'prefer-rest-params': 'error',
          'prefer-spread': 'error',
        },
      },
    ],
    options: {
      typeAware: true,
      typeCheck: true,
    },
  },
  base: './',
  // 开发服务器选项 https://cn.vitejs.dev/config/#server-options
  server: {
    open: true,
    port: 9000,
    // host: '0.0.0.0',
    proxy: {
      '/proxy': {
        target: env.VITE_APP_API_BASEURL,
        changeOrigin: command === 'serve' && env.VITE_OPEN_PROXY === 'true',
        rewrite: (path: string) => path.replace(/\/proxy/, ''),
      },
    },
  },
  // 构建选项 https://cn.vitejs.dev/config/#server-fsserve-root
  build: {
    outDir: mode === 'production' ? 'dist' : `dist-${mode}`,
    sourcemap: env.VITE_BUILD_SOURCEMAP === 'true',
  },
  define: {
    __SYSTEM_INFO__: JSON.stringify({
      pkg: {
        version: pkg.version,
        dependencies: pkg.dependencies,
        devDependencies: pkg.devDependencies,
      },
      lastBuildTime: dayjs().format('YYYY-MM-DD HH:mm:ss'),
    }),
  },
  plugins: createVitePlugins(env, command === 'build'),
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
      '#': fileURLToPath(new URL('./src/types', import.meta.url)),
    },
  },
  css: {
    preprocessorOptions: {
      scss: {
        additionalData: scssResources.join(''),
      },
    },
  },
})
