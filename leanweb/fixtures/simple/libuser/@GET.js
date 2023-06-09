const {myFunc} = require("/lib/mylib.js")
function handler(w,r) {
    w.write(myFunc())
}