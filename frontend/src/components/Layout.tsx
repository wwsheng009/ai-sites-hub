import { useEffect, useState } from 'react'
import type { ReactNode } from 'react'
import { NavLink, Link } from 'react-router-dom'

// 图标：内联 SVG（heroicons 风格），避免引入图标库依赖
type IconProps = { className?: string }
const iconClass = 'h-5 w-5 flex-shrink-0'

const SquaresIcon = ({ className = iconClass }: IconProps) => (
  <svg className={className} fill="none" viewBox="0 0 24 24" strokeWidth={1.8} stroke="currentColor">
    <path strokeLinecap="round" strokeLinejoin="round" d="M3.75 6A2.25 2.25 0 016 3.75h2.25A2.25 2.25 0 0110.5 6v2.25a2.25 2.25 0 01-2.25 2.25H6a2.25 2.25 0 01-2.25-2.25V6zM3.75 15.75A2.25 2.25 0 016 13.5h2.25a2.25 2.25 0 012.25 2.25V18a2.25 2.25 0 01-2.25 2.25H6A2.25 2.25 0 013.75 18v-2.25zM13.5 6a2.25 2.25 0 012.25-2.25H18A2.25 2.25 0 0120.25 6v2.25A2.25 2.25 0 0118 10.5h-2.25a2.25 2.25 0 01-2.25-2.25V6zM13.5 15.75a2.25 2.25 0 012.25-2.25H18a2.25 2.25 0 012.25 2.25V18A2.25 2.25 0 0118 20.25h-2.25A2.25 2.25 0 0113.5 18v-2.25z" />
  </svg>
)

const GlobeIcon = ({ className = iconClass }: IconProps) => (
  <svg className={className} fill="none" viewBox="0 0 24 24" strokeWidth={1.8} stroke="currentColor">
    <path strokeLinecap="round" strokeLinejoin="round" d="M12 21a9.004 9.004 0 008.716-6.747M12 21a9.004 9.004 0 01-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 017.843 4.582M12 3a8.997 8.997 0 00-7.843 4.582m15.686 0A11.953 11.953 0 0112 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0121 12c0 .778-.099 1.533-.284 2.253m-18.716 0A13.076 13.076 0 009.5 9.25m0 0a11.98 11.98 0 005 0m-5 0a13.076 13.076 0 01-2.5-2.25" />
  </svg>
)

const CurrencyIcon = ({ className = iconClass }: IconProps) => (
  <svg className={className} fill="none" viewBox="0 0 24 24" strokeWidth={1.8} stroke="currentColor">
    <path strokeLinecap="round" strokeLinejoin="round" d="M12 6v12m-3-2.818l.879.659c1.171.879 3.07.879 4.242 0 1.172-.879 1.172-2.303 0-3.182C13.536 12.219 12.768 12 12 12c-.725 0-1.45-.22-2.003-.659-1.106-.879-1.106-2.303 0-3.182s2.9-.879 4.006 0l.415.33M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
  </svg>
)

const SunIcon = ({ className = iconClass }: IconProps) => (
  <svg className={className} fill="none" viewBox="0 0 24 24" strokeWidth={1.8} stroke="currentColor">
    <path strokeLinecap="round" strokeLinejoin="round" d="M12 3v2.25m6.364.386l-1.591 1.591M21 12h-2.25m-.386 6.364l-1.591-1.591M12 18.75V21m-4.773-4.227l-1.591 1.591M5.25 12H3m4.227-4.773L5.636 5.636M15.75 12a3.75 3.75 0 11-7.5 0 3.75 3.75 0 017.5 0z" />
  </svg>
)

const MoonIcon = ({ className = iconClass }: IconProps) => (
  <svg className={className} fill="none" viewBox="0 0 24 24" strokeWidth={1.8} stroke="currentColor">
    <path strokeLinecap="round" strokeLinejoin="round" d="M21.752 15.002A9.718 9.718 0 0118 15.75c-5.385 0-9.75-4.365-9.75-9.75 0-1.33.266-2.597.748-3.752A9.753 9.753 0 003 11.25C3 16.635 7.365 21 12.75 21a9.753 9.753 0 009.002-5.998z" />
  </svg>
)

const MenuIcon = ({ className = iconClass }: IconProps) => (
  <svg className={className} fill="none" viewBox="0 0 24 24" strokeWidth={1.8} stroke="currentColor">
    <path strokeLinecap="round" strokeLinejoin="round" d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25h16.5" />
  </svg>
)

const ChevronIcon = ({ className = iconClass }: IconProps) => (
  <svg className={className} fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor">
    <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 19.5L8.25 12l7.5-7.5" />
  </svg>
)

// 主题管理：跟随 localStorage，跨组件同步
function useTheme() {
  const [dark, setDark] = useState(() => localStorage.getItem('theme') === 'dark')
  useEffect(() => {
    document.documentElement.classList.toggle('dark', dark)
    localStorage.setItem('theme', dark ? 'dark' : 'light')
  }, [dark])
  return { dark, toggle: () => setDark((d) => !d) }
}

const navItems = [
  { to: '/', label: '概览', icon: SquaresIcon, end: true },
  { to: '/sites', label: '站点管理', icon: GlobeIcon, end: false },
  { to: '/affiliates', label: '返利中心', icon: CurrencyIcon, end: false },
]

export default function Layout({ children }: { children: ReactNode }) {
  const { dark, toggle } = useTheme()
  const [collapsed, setCollapsed] = useState(() => localStorage.getItem('sidebar-collapsed') === '1')
  const [mobileOpen, setMobileOpen] = useState(false)

  useEffect(() => {
    localStorage.setItem('sidebar-collapsed', collapsed ? '1' : '0')
  }, [collapsed])

  // 路由切换时关闭移动端抽屉
  useEffect(() => {
    const close = () => setMobileOpen(false)
    window.addEventListener('hashchange', close)
    return () => window.removeEventListener('hashchange', close)
  }, [])

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-dark-950">
      {/* 背景装饰（mesh-gradient，同 sub2api AppLayout） */}
      <div className="pointer-events-none fixed inset-0 bg-mesh-gradient" />

      {/* 移动端遮罩 */}
      {mobileOpen && (
        <div className="fixed inset-0 z-40 bg-dark-950/50 backdrop-blur-sm lg:hidden" onClick={() => setMobileOpen(false)} />
      )}

      {/* 侧边栏 */}
      <aside
        className={`sidebar ${collapsed ? 'w-[72px]' : 'w-64'} ${
          mobileOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'
        }`}
      >
        {/* Logo / Brand */}
        <div className={`sidebar-header ${collapsed ? 'justify-center px-2' : ''}`}>
          <Link to="/" className="sidebar-logo">
            A
          </Link>
          {!collapsed && (
            <div className="min-w-0">
              <div className="sidebar-brand-title">ai-sites-client</div>
              <div className="text-xs text-gray-400 dark:text-dark-400">AI 站点集中管理</div>
            </div>
          )}
        </div>

        {/* 导航 */}
        <nav className="sidebar-nav">
          <div className="sidebar-section-title">{collapsed ? '·' : '导航'}</div>
          {navItems.map((item) => {
            const Icon = item.icon
            return (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.end}
                onClick={() => setMobileOpen(false)}
                className={({ isActive }) => `sidebar-link mb-1 ${isActive ? 'sidebar-link-active' : ''} ${collapsed ? 'justify-center' : ''}`}
                title={collapsed ? item.label : undefined}
              >
                <Icon />
                {!collapsed && <span className="truncate">{item.label}</span>}
              </NavLink>
            )
          })}
        </nav>

        {/* 底部：折叠按钮 */}
        <div className="sidebar-footer">
          <button
            type="button"
            onClick={() => setCollapsed((c) => !c)}
            className={`sidebar-link w-full ${collapsed ? 'justify-center' : ''}`}
            title={collapsed ? '展开侧边栏' : '收起侧边栏'}
          >
            <ChevronIcon className={`h-5 w-5 flex-shrink-0 transition-transform duration-300 ${collapsed ? '' : 'rotate-180'}`} />
            {!collapsed && <span>收起</span>}
          </button>
        </div>
      </aside>

      {/* 主内容区 */}
      <div className={`relative min-h-screen transition-all duration-300 ${collapsed ? 'lg:ml-[72px]' : 'lg:ml-64'}`}>
        {/* 顶栏 */}
        <header className="app-header">
          <button className="btn btn-ghost btn-icon lg:hidden" onClick={() => setMobileOpen(true)} aria-label="打开菜单">
            <MenuIcon className="h-5 w-5" />
          </button>
          <div className="text-sm font-medium text-gray-500 dark:text-dark-300">AI 站点集中管理控制台</div>
          <div className="ml-auto flex items-center gap-2">
            <button
              type="button"
              onClick={toggle}
              className="btn btn-ghost btn-icon"
              aria-label="切换暗色模式"
              title={dark ? '切换到亮色模式' : '切换到暗色模式'}
            >
              {dark ? <SunIcon className="h-5 w-5" /> : <MoonIcon className="h-5 w-5" />}
            </button>
          </div>
        </header>

        <main className="mx-auto max-w-7xl p-4 md:p-6 lg:p-8 animate-fade-in">{children}</main>
      </div>
    </div>
  )
}
