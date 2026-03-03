import DefaultTheme from 'vitepress/theme'
import type { Theme } from 'vitepress'
import CategorySwitcher from './components/CategorySwitcher.vue'
import './custom.css'

const theme: Theme = {
  ...DefaultTheme,
  enhanceApp({ app }) {
    app.component('CategorySwitcher', CategorySwitcher)
  },
  Layout: DefaultTheme.Layout
}

export default theme
