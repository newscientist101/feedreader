export default [
  {
    ignores: ["srv/static/**/*.test.js", "srv/static/test-helper.js"],
  },
  {
    files: ["srv/static/**/*.js"],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: "script",
      globals: {
        // Browser globals
        window: "readonly",
        document: "readonly",
        console: "readonly",
        fetch: "readonly",
        localStorage: "readonly",
        sessionStorage: "readonly",
        setTimeout: "readonly",
        setInterval: "readonly",
        clearTimeout: "readonly",
        clearInterval: "readonly",
        requestAnimationFrame: "readonly",
        cancelAnimationFrame: "readonly",
        navigator: "readonly",
        location: "readonly",
        history: "readonly",
        URLSearchParams: "readonly",
        URL: "readonly",
        HTMLElement: "readonly",
        Node: "readonly",
        Event: "readonly",
        KeyboardEvent: "readonly",
        MouseEvent: "readonly",
        MutationObserver: "readonly",
        IntersectionObserver: "readonly",
        ResizeObserver: "readonly",
        FormData: "readonly",
        AbortController: "readonly",
        DOMParser: "readonly",
        alert: "readonly",
        confirm: "readonly",
        prompt: "readonly",
        getComputedStyle: "readonly",
        scrollTo: "readonly",
        // Service Worker
        self: "readonly",
        caches: "readonly",
        clients: "readonly",
        Response: "readonly",
        Request: "readonly",
      },
    },
    rules: {
      // Possible errors
      "no-dupe-args": "error",
      "no-dupe-keys": "error",
      "no-duplicate-case": "error",
      "no-unreachable": "error",
      "no-unexpected-multiline": "error",
      "no-constant-condition": "warn",
      "no-empty": ["warn", { allowEmptyCatch: true }],

      // Best practices
      "eqeqeq": ["warn", "smart"],
      "no-eval": "error",
      "no-implied-eval": "error",
      "no-self-assign": "error",
      "no-self-compare": "error",
      "no-redeclare": "error",
      "no-unused-vars": ["warn", {
        args: "none",
        caughtErrors: "none",
        varsIgnorePattern: "^(applyHideEmptyFeeds|applyHideReadArticles|closeCreateFolderModal|copyNewsletterAddress|deleteCategory|deleteFeed|exportOPML|filterFeeds|generateNewsletterAddress|importOPML|markAsRead|markRead|markUnread|navigateFolder|openArticle|openArticleExternal|openCreateFolderModal|renameCategory|runCleanup|saveFeed|setFeedCategory|showHiddenArticles|showReadArticles|submitCreateFolder|toggleDropdown|toggleFolderCollapse|toggleQueue|toggleStar|unparentCategory)$",
      }],
      "no-undef": "error",
      "no-use-before-define": ["warn", { functions: false, classes: false }],
    },
  },
  // ES module override for app.js and files in the modules/ directory
  {
    files: ["srv/static/app.js", "srv/static/modules/**/*.js"],
    languageOptions: {
      sourceType: "module",
    },
    rules: {
      "no-unused-vars": ["warn", { args: "none", caughtErrors: "none" }],
    },
  },
];
