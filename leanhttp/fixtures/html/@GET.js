function handler(w,r) {
    const response = requestHttp("GET",testServerUrl)
    w.write(response.body)
}