function handler(w,r) {
    w.write(renderToString("index",{foo: "bar"}))
}