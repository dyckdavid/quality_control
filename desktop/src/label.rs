//! Printable shop labels: a QR code holding the record's code, with the code and name in type beside it.

use ab_glyph::{point, Font, FontRef, PxScale, ScaleFont};
use image::{GrayImage, Luma};
use qrcode::{Color, EcLevel, QrCode};

const WIDTH: u32 = 900;
const HEIGHT: u32 = 420;
const MARGIN: u32 = 30;

/// Renders a 2 by 1 label as PNG bytes. The QR payload is the plain code, so any scanner reads it.
pub fn render_png(code: &str, title: &str, subtitle: &str) -> Result<Vec<u8>, String> {
    let qr = QrCode::with_error_correction_level(code.as_bytes(), EcLevel::M).map_err(|e| e.to_string())?;
    let modules = qr.width() as u32;
    let quiet = 2;
    let side = HEIGHT - 2 * MARGIN;
    let cell = (side / (modules + 2 * quiet)).max(1);
    let qr_px = cell * (modules + 2 * quiet);
    let top = (HEIGHT - qr_px) / 2;

    let mut img = GrayImage::from_pixel(WIDTH, HEIGHT, Luma([255]));
    for (i, color) in qr.to_colors().iter().enumerate() {
        if *color != Color::Dark {
            continue;
        }
        let (mx, my) = ((i as u32) % modules, (i as u32) / modules);
        let (x0, y0) = (MARGIN + (mx + quiet) * cell, top + (my + quiet) * cell);
        for y in y0..y0 + cell {
            for x in x0..x0 + cell {
                img.put_pixel(x, y, Luma([0]));
            }
        }
    }

    let bold = FontRef::try_from_slice(include_bytes!("../assets/Inter-SemiBold.ttf")).map_err(|e| e.to_string())?;
    let regular = FontRef::try_from_slice(include_bytes!("../assets/Inter-Regular.ttf")).map_err(|e| e.to_string())?;
    let text_x = (MARGIN + qr_px + 24) as f32;
    let room = WIDTH as f32 - text_x - MARGIN as f32;
    let code_size = fit(&bold, code, 96.0, room);
    draw(&mut img, &bold, code, code_size, text_x, 150.0);
    draw(&mut img, &regular, &clip_to(&regular, title, 40.0, room), 40.0, text_x, 230.0);
    draw(&mut img, &regular, &clip_to(&regular, subtitle, 32.0, room), 32.0, text_x, 290.0);

    let mut out = Vec::new();
    image::DynamicImage::ImageLuma8(img)
        .write_to(&mut std::io::Cursor::new(&mut out), image::ImageFormat::Png)
        .map_err(|e| e.to_string())?;
    Ok(out)
}

fn width(font: &FontRef, text: &str, size: f32) -> f32 {
    let scaled = font.as_scaled(PxScale::from(size));
    text.chars().map(|c| scaled.h_advance(scaled.glyph_id(c))).sum()
}

/// The largest size up to `max` at which `text` fits in `room` pixels.
fn fit(font: &FontRef, text: &str, max: f32, room: f32) -> f32 {
    let w = width(font, text, max);
    if w <= room || w == 0.0 {
        max
    } else {
        (max * room / w).max(18.0)
    }
}

fn clip_to(font: &FontRef, text: &str, size: f32, room: f32) -> String {
    if width(font, text, size) <= room {
        return text.to_string();
    }
    let mut out: String = text.chars().collect();
    while !out.is_empty() && width(font, &format!("{out}…"), size) > room {
        out.pop();
    }
    format!("{}…", out.trim_end())
}

/// Draws one line with its baseline at `baseline`.
fn draw(img: &mut GrayImage, font: &FontRef, text: &str, size: f32, x: f32, baseline: f32) {
    let scaled = font.as_scaled(PxScale::from(size));
    let mut caret = x;
    for c in text.chars() {
        let id = scaled.glyph_id(c);
        let glyph = id.with_scale_and_position(PxScale::from(size), point(caret, baseline));
        caret += scaled.h_advance(id);
        let Some(outlined) = font.outline_glyph(glyph) else {
            continue;
        };
        let bounds = outlined.px_bounds();
        outlined.draw(|gx, gy, coverage| {
            let px = bounds.min.x as i32 + gx as i32;
            let py = bounds.min.y as i32 + gy as i32;
            if px < 0 || py < 0 || px >= img.width() as i32 || py >= img.height() as i32 {
                return;
            }
            let cur = img.get_pixel(px as u32, py as u32).0[0] as f32;
            let ink = cur * (1.0 - coverage.clamp(0.0, 1.0));
            img.put_pixel(px as u32, py as u32, Luma([ink as u8]));
        });
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn label_is_a_png_with_dark_modules() {
        let png = render_png("CHUTE-101", "Squeeze chute", "1 ea").unwrap();
        assert_eq!(&png[1..4], b"PNG");
        let img = image::load_from_memory(&png).unwrap().to_luma8();
        assert_eq!((img.width(), img.height()), (WIDTH, HEIGHT));
        assert!(img.pixels().any(|p| p.0[0] == 0));
    }

    #[test]
    fn long_names_are_clipped() {
        let font = FontRef::try_from_slice(include_bytes!("../assets/Inter-Regular.ttf")).unwrap();
        let clipped = clip_to(&font, &"Heavy duty corral panel ".repeat(6), 40.0, 400.0);
        assert!(clipped.ends_with('…'));
        assert!(width(&font, &clipped, 40.0) <= 400.0);
    }
}
