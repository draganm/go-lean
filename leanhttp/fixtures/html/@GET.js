function handler(w,r) {
    const response = http.request("GET",testServerUrl)
    w.Write(response.body)
}