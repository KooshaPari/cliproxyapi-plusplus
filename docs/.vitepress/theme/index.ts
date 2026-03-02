import DefaultTheme from 'vitepress/theme'
import type { Theme } from 'vitepress'
import './custom.css'

const theme: Theme = {
  ...DefaultTheme,
  Layout: DefaultTheme.Layout
}

export default theme
