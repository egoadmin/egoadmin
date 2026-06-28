import path from 'node:path'
import fs from 'node:fs'
import { fileURLToPath } from 'node:url'
import { icons } from '@iconify-json/ep'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const outputDir = path.resolve(__dirname, '../public/icons')

fs.mkdirSync(outputDir, { recursive: true })
for (const file of fs.readdirSync(outputDir)) {
  fs.unlinkSync(path.join(outputDir, file))
}
fs.writeFileSync(path.join(outputDir, 'icons.json'), JSON.stringify(icons))
