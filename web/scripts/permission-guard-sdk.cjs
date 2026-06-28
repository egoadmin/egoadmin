const fs = require('fs')
const path = require('path')

function cleanApiPath(api) {
  if (!api || typeof api !== 'string') {
    return ''
  }
  let cleaned = api.trim()
  if (cleaned.startsWith('/')) {
    cleaned = cleaned.substring(1)
  }
  return cleaned.toUpperCase()
}

function generateContract({ menuTree, mapNode, childrenKey = 'children', outputPath }) {
  if (!Array.isArray(menuTree)) {
    throw new Error('generateContract: menuTree must be an array')
  }
  if (typeof mapNode !== 'function') {
    throw new Error('generateContract: mapNode must be a function')
  }
  if (!outputPath) {
    throw new Error('generateContract: outputPath is required')
  }

  const contract = {}

  function traverse(nodes, parentNames = []) {
    if (!Array.isArray(nodes)) {
      return
    }

    for (const node of nodes) {
      const { key, apis } = mapNode(node)
      const title = node.title || node.name || ''
      const currentNames = title ? [...parentNames, title] : parentNames
      const children = node[childrenKey]
      const apiSet = new Set()

      if (apis) {
        if (Array.isArray(apis)) {
          apis.forEach((api) => {
            const cleaned = cleanApiPath(api)
            if (cleaned) {
              apiSet.add(cleaned)
            }
          })
        } else if (typeof apis === 'string') {
          apis.split(',').forEach((api) => {
            const cleaned = cleanApiPath(api)
            if (cleaned) {
              apiSet.add(cleaned)
            }
          })
        }
      }

      if (key !== undefined && key !== null && key !== '' && apiSet.size > 0) {
        const stringKey = String(key)
        if (!contract[stringKey]) {
          contract[stringKey] = {
            name: currentNames.join(' - '),
            apis: [],
          }
        }
        apiSet.forEach((api) => {
          if (!contract[stringKey].apis.includes(api)) {
            contract[stringKey].apis.push(api)
          }
        })
      }

      if (Array.isArray(children) && children.length > 0) {
        traverse(children, currentNames)
      }
    }
  }

  traverse(menuTree)

  const dir = path.dirname(outputPath)
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true })
  }

  fs.writeFileSync(outputPath, JSON.stringify(contract, null, 2), 'utf-8')
  console.log(`[Permission Guard SDK] contract generated: ${path.resolve(outputPath)}`)
}

module.exports = {
  generateContract,
  cleanApiPath,
}
