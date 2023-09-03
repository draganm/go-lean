function handler(w, r) {
  log.Info("foo", "bar", "baz")
  w.Write('foo')
}
