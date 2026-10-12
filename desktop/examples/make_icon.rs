//! Draws the app icon: a teal tile with scan-frame corners and a check mark.
//! Run `cargo run --example make_icon` from `desktop/` to rewrite `assets/icon.png` and `assets/icon.ico`.

use image::codecs::ico::{IcoEncoder, IcoFrame};
use image::imageops::FilterType;
use image::{ExtendedColorType, ImageEncoder, Rgba, RgbaImage};
use std::path::Path;

const SIZE: u32 = 1024;

fn main() {
    let icon = draw(SIZE);
    let assets = Path::new(env!("CARGO_MANIFEST_DIR")).join("assets");
    icon.save(assets.join("icon.png")).expect("write icon.png");

    let frames: Vec<IcoFrame> = [16, 24, 32, 48, 64, 128, 256]
        .iter()
        .map(|&side| {
            let small = image::imageops::resize(&icon, side, side, FilterType::Lanczos3);
            let mut png = Vec::new();
            image::codecs::png::PngEncoder::new(&mut png)
                .write_image(small.as_raw(), side, side, ExtendedColorType::Rgba8)
                .expect("encode frame");
            IcoFrame::with_encoded(png, side, side, ExtendedColorType::Rgba8).expect("ico frame")
        })
        .collect();
    let file = std::fs::File::create(assets.join("icon.ico")).expect("create icon.ico");
    IcoEncoder::new(file).encode_images(&frames).expect("write icon.ico");
    println!("wrote {}", assets.display());
}

fn draw(size: u32) -> RgbaImage {
    let s = size as f32;
    // macOS icon grid: the tile fills about 80% of the canvas, leaving room for the system shadow.
    let half = s * 0.40;
    let radius = s * 0.18;
    let center = s / 2.0;
    let top = [45.0, 168.0, 148.0];
    let bottom = [15.0, 110.0, 98.0];

    let stroke = s * 0.035;
    let frame = s * 0.25;
    let arm = s * 0.10;
    let corners: Vec<([f32; 2], [f32; 2])> = [(-1.0, -1.0), (1.0, -1.0), (-1.0, 1.0), (1.0, 1.0)]
        .iter()
        .flat_map(|&(dx, dy): &(f32, f32)| {
            let corner = [center + dx * frame, center + dy * frame];
            [
                (corner, [corner[0] - dx * arm, corner[1]]),
                (corner, [corner[0], corner[1] - dy * arm]),
            ]
        })
        .collect();
    let check = [
        ([center - s * 0.115, center + s * 0.005], [center - s * 0.03, center + s * 0.09]),
        ([center - s * 0.03, center + s * 0.09], [center + s * 0.13, center - s * 0.085]),
    ];

    RgbaImage::from_fn(size, size, |x, y| {
        let p = [x as f32 + 0.5, y as f32 + 0.5];
        let tile = coverage(round_box(p, center, half, radius));
        if tile <= 0.0 {
            return Rgba([0, 0, 0, 0]);
        }
        let t = ((p[1] - (center - half)) / (2.0 * half)).clamp(0.0, 1.0);
        let mut rgb = [0.0; 3];
        for i in 0..3 {
            rgb[i] = top[i] + (bottom[i] - top[i]) * t;
        }
        let marks = corners
            .iter()
            .chain(check.iter())
            .map(|(a, b)| segment(p, *a, *b) - stroke)
            .fold(f32::MAX, f32::min);
        let white = coverage(marks);
        for c in &mut rgb {
            *c += (255.0 - *c) * white;
        }
        Rgba([rgb[0] as u8, rgb[1] as u8, rgb[2] as u8, (tile * 255.0) as u8])
    })
}

fn coverage(distance: f32) -> f32 {
    (0.5 - distance).clamp(0.0, 1.0)
}

fn round_box(p: [f32; 2], center: f32, half: f32, radius: f32) -> f32 {
    let qx = (p[0] - center).abs() - (half - radius);
    let qy = (p[1] - center).abs() - (half - radius);
    let outside = (qx.max(0.0).powi(2) + qy.max(0.0).powi(2)).sqrt();
    outside + qx.max(qy).min(0.0) - radius
}

fn segment(p: [f32; 2], a: [f32; 2], b: [f32; 2]) -> f32 {
    let (pa, ba) = ([p[0] - a[0], p[1] - a[1]], [b[0] - a[0], b[1] - a[1]]);
    let h = ((pa[0] * ba[0] + pa[1] * ba[1]) / (ba[0] * ba[0] + ba[1] * ba[1])).clamp(0.0, 1.0);
    ((pa[0] - ba[0] * h).powi(2) + (pa[1] - ba[1] * h).powi(2)).sqrt()
}
