import { createI18n } from 'vue-i18n'

const messages = {
  en: {
    // Login
    login: {
      title: 'Sign in to your account',
      username: 'Username',
      password: 'Password',
      signIn: 'Sign in',
      signingIn: 'Signing in...',
    },
    // Sidebar
    sidebar: {
      dashboard: 'Dashboard',
      sites: 'Sites',
      services: 'Services',
      settings: 'Settings',
      modules: 'Modules',
      firewall: 'Firewall',
      signOut: 'Sign out',
    },
    // Settings & Updater
    settings: {
      title: 'Settings',
      subtitle: 'Application configuration',
      general: 'General',
      panelTitle: 'Panel Title',
      defaultPhp: 'Default PHP Version',
      saveSettings: 'Save Settings',
      saving: 'Saving...',
      saved: 'Settings saved!',
      about: 'About',
      version: 'Version',
      license: 'License',
      github: 'GitHub',
      updates: {
        title: 'Project updates',
        desc: 'Check published releases from anrted/opendeploy on GitHub.',
        check: 'Check for updates',
        checking: 'Checking...',
        installed: 'Installed',
        latest: 'Latest release',
        available: 'Update available',
        upToDate: 'Up to date',
        updateNow: 'Update now',
        updating: 'Updating...',
        viewChanges: 'View changes',
        started: 'Update started. The panel will reload after services restart.',
        timeout: 'Automatic update check timed out (possibly running locally without systemd). Please update manually.',
        cancel: 'Cancel',
      }
    },
    // Sites
    sites: {
      fileManager: {
        title: 'File Manager',
        upload: 'Upload File',
        name: 'Name',
        size: 'Size',
        modified: 'Modified',
        loading: 'Loading...',
        empty: 'Directory is empty',
      }
    }
  },
  ru: {
    // Login
    login: {
      title: 'Войдите в свой аккаунт',
      username: 'Имя пользователя',
      password: 'Пароль',
      signIn: 'Войти',
      signingIn: 'Вход...',
    },
    // Sidebar
    sidebar: {
      dashboard: 'Дашборд',
      sites: 'Сайты',
      services: 'Сервисы',
      settings: 'Настройки',
      modules: 'Модули',
      firewall: 'Файрвол',
      signOut: 'Выйти',
    },
    // Settings & Updater
    settings: {
      title: 'Настройки',
      subtitle: 'Конфигурация приложения',
      general: 'Общие',
      panelTitle: 'Название панели',
      defaultPhp: 'Версия PHP по умолчанию',
      saveSettings: 'Сохранить настройки',
      saving: 'Сохранение...',
      saved: 'Настройки сохранены!',
      about: 'О программе',
      version: 'Версия',
      license: 'Лицензия',
      github: 'GitHub',
      updates: {
        title: 'Обновления проекта',
        desc: 'Проверка опубликованных релизов от anrted/opendeploy на GitHub.',
        check: 'Проверить обновления',
        checking: 'Проверка...',
        installed: 'Установлено',
        latest: 'Последний релиз',
        available: 'Доступно обновление',
        upToDate: 'Обновлений нет',
        updateNow: 'Обновить сейчас',
        updating: 'Обновление...',
        viewChanges: 'Посмотреть изменения',
        started: 'Обновление запущено. Панель перезагрузится после перезапуска сервисов.',
        timeout: 'Ожидание автоматического обновления истекло (возможно, запуск локально без systemd). Обновите панель вручную.',
        cancel: 'Отмена',
      }
    },
    // Sites
    sites: {
      fileManager: {
        title: 'Файловый менеджер',
        upload: 'Загрузить файл',
        name: 'Имя',
        size: 'Размер',
        modified: 'Изменен',
        loading: 'Загрузка...',
        empty: 'Папка пуста',
      }
    }
  }
}

// Check local storage or browser language for default
const savedLocale = localStorage.getItem('opendeploy_locale')
const defaultLocale = savedLocale || (navigator.language.split('-')[0] === 'ru' ? 'ru' : 'en')

const i18n = createI18n({
  legacy: false, // use Composition API
  locale: defaultLocale,
  fallbackLocale: 'en',
  messages,
})

export default i18n
