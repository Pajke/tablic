// Debug logging is enabled in dev mode or when ?debug is present in the URL.
// Usage: if (DEBUG) console.log(...)
export const DEBUG: boolean =
  import.meta.env.DEV || new URLSearchParams(location.search).has('debug')
