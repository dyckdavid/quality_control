fn main() {
    println!("cargo:rerun-if-changed=assets/icon.ico");
    #[cfg(windows)]
    {
        let mut res = winresource::WindowsResource::new();
        res.set_icon("assets/icon.ico")
            .set("ProductName", "Quality Control")
            .set("FileDescription", "Quality Control");
        if let Err(err) = res.compile() {
            println!("cargo:warning=could not embed the Windows icon: {err}");
        }
    }
}
