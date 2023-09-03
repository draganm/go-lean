function handler(w,r) {
    w.Write(JSON.stringify(sql.query("select * FROM (select count(*) as count, 'abc' from blog UNION select count(*)+1 as count, 'def' from blog) ORDER BY count",[],(it) => Array.from(it, (x) => x)) ))

}