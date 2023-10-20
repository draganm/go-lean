function handler(w,r) {
    w.Write(mustache.renderToString("index",{foo: "bar"}))
}