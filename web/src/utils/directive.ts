import type { App } from 'vue'

function getStyle(el: HTMLElement, styleName: string) {
  return window.getComputedStyle(el).getPropertyValue(styleName)
}

export default function directive(app: App) {
  // 注册 v-auth 和 v-auth-all 指令
  app.directive('auth', {
    mounted: (el, binding) => {
      if (!useAuth().auth(binding.value)) {
        el.remove()
      }
    },
  })
  app.directive('auth-all', {
    mounted: (el, binding) => {
      if (!useAuth().authAll(binding.value)) {
        el.remove()
      }
    },
  })
  app.directive('show-tip', {
    created(el, binding, vnode: any) {
      const tooltipNode = vnode.children.find(
        (childCmpt: any) => childCmpt.component?.type.name === 'ElTooltip',
      )
      if (tooltipNode) {
        // let { content } = tooltipNode.props;
        el.addEventListener('mouseenter', () => {
          tooltipNode.component.props.disabled = true
          const range = document.createRange()
          range.setStart(el, 0)
          range.setEnd(el, el.childNodes.length)
          const rangeWidth = Math.round(range.getBoundingClientRect().width)
          const padding =
            (parseInt(getStyle(el, 'paddingLeft'), 10) || 0) +
            (parseInt(getStyle(el, 'paddingRight'), 10) || 0)
          if (rangeWidth + padding > el.offsetWidth || el.scrollWidth > el.offsetWidth) {
            tooltipNode.component.props.disabled = false
          }
        })
      }
    },
  })
  app.directive('blur', {
    mounted: (el) => {
      el.addEventListener('focus', () => {
        el.blur()
      })
    },
  })
}
