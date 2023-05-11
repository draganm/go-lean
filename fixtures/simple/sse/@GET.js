function handler(w,r) {
    let cnt = 0
    return sendServerEvents(() => {
        if (cnt < 1) {
            cnt++
            return {id: `${cnt}`, event: "foo", data: "bar"}
        }
        return null
    })
}