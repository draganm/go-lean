function handler(w,r) {
    w.write(mustache.renderToString("index",{foo: "bar"}))
}