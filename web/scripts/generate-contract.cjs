const fs = require('fs')
const path = require('path')
const { generateContract } = require('./permission-guard-sdk.cjs')

const routeMenuPath = path.resolve(__dirname, '../src/config/routeMenu.ts')
if (!fs.existsSync(routeMenuPath)) {
  console.error(`[Error] routeMenu.ts not found: ${routeMenuPath}`)
  process.exit(1)
}

let content = fs.readFileSync(routeMenuPath, 'utf-8')

const apiManifestPath = path.resolve(__dirname, '../src/api/api-manifest.ts')
let apisObjStr = '{}'
if (fs.existsSync(apiManifestPath)) {
  const manifestContent = fs.readFileSync(apiManifestPath, 'utf-8')
  const match = manifestContent.match(/export\s+const\s+APIs\s*=\s*([\s\S]*?)\s*as\s+const/)
  if (match) {
    apisObjStr = match[1]
  }
}

content = content.replace(/import\s+[\s\S]*?from\s+['"].*?['"];?/g, '')
content = content.replace(/interface\s+\w+\s*\{[\s\S]*?\}/g, '')
content = content.replace(/export\s+const\s+menu\s*(:\s*[\w[\]<>]+)?\s*=\s*/g, 'const menu = ')
content = `const APIs = ${apisObjStr};\n` + content
content += '\nmodule.exports = { menu };'

const tempFile = path.resolve(__dirname, './temp-menu.cjs')
fs.writeFileSync(tempFile, content, 'utf-8')

try {
  const { menu } = require(tempFile)

  generateContract({
    menuTree: menu,
    childrenKey: 'child',
    mapNode: (node) => ({
      key: node.id,
      apis: node.apiList,
    }),
    outputPath: path.resolve(__dirname, '../dist/permission-contract.json'),
  })
} catch (err) {
  console.error('[Error] permission contract generation failed:', err)
  process.exit(1)
} finally {
  if (fs.existsSync(tempFile)) {
    fs.unlinkSync(tempFile)
  }
}
