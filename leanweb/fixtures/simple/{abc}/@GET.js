function handler(w, r) {
  log.info("foo", "bar", "baz")
  w.write('foo')
}
