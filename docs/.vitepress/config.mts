import { defineConfig } from 'vitepress'

// https://vitepress.dev/reference/site-config
export default defineConfig({
  title: "ReEscape Protocol",
  description: "基于 OneBot WebSocket 的可扩展聊天机器人运行时",
  themeConfig: {
    // https://vitepress.dev/reference/default-theme-config
    nav: [
      { text: '首页', link: '/' },
      { text: '快速开始', link: '/quickstart' },
      { text: '开发指南', link: '/development' }
    ],

    sidebar: {
      '/': [
        {
          text: '使用文档',
          items: [
            { text: '快速开始', link: '/quickstart' },
            { text: '配置参考', link: '/configuration' },
            { text: '架构概览', link: '/architecture' }
          ]
        },
        {
          text: '参与项目',
          items: [
            { text: '开发指南', link: '/development' },
            { text: '安全报告', link: '/security' }
          ]
        }
      ]
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/' }
    ]
  }
})
