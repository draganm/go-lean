({
    schedule: "* * * * * *",
    allowParallel: true,
    run: function() {
        close()
    }
})