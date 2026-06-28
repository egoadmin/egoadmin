import type { Menu } from '#/global'

const menus: Menu.recordRaw = {
  meta: {
    title: 'menu.multilevel',
    icon: 'sidebar-menu',
  },
  children: [
    {
      path: '/multilevel_menu_example/page',
      meta: {
        title: 'menu.navigation1',
      },
    },
    {
      meta: {
        title: 'menu.navigation2',
      },
      children: [
        {
          path: '/multilevel_menu_example/level2/page',
          meta: {
            title: 'menu.navigation2_1',
          },
        },
        {
          meta: {
            title: 'menu.navigation2_2',
          },
          children: [
            {
              path: '/multilevel_menu_example/level2/level3/page1',
              meta: {
                title: 'menu.navigation2_2_1',
              },
            },
            {
              path: '/multilevel_menu_example/level2/level3/page2',
              meta: {
                title: 'menu.navigation2_2_2',
              },
            },
          ],
        },
      ],
    },
  ],
}

export default menus
