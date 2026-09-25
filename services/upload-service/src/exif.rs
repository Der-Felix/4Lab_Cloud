use chrono::NaiveDateTime;
use serde::{Deserialize, Serialize};
use std::io::Cursor;

/// GPS-Koordinaten und Hoehe aus EXIF-Metadaten.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ExifGps {
    pub lat: f64,
    pub lon: f64,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub altitude: Option<f64>,
}

/// Strukturierte EXIF-Metadaten eines Fotos.
#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct ExifData {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub make: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub model: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub lens_model: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub exposure_time: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub f_number: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub iso: Option<u32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub focal_length: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub datetime_original: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub gps: Option<ExifGps>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub orientation: Option<u32>,
}

/// Rechnet DMS (Grad, Minuten, Sekunden) in Dezimalgrad um.
pub fn dms_to_decimal(degrees: f64, minutes: f64, seconds: f64, ref_char: &str) -> f64 {
    let dec = degrees + (minutes / 60.0) + (seconds / 3600.0);
    if ref_char.eq_ignore_ascii_case("S") || ref_char.eq_ignore_ascii_case("W") {
        -dec
    } else {
        dec
    }
}

/// Parst Rationals fuer GPS-Koordinaten aus EXIF.
///
/// kamadak-exif's `Rational::to_f64` fuehrt keine Nenner-Null-Pruefung durch
/// (schlicht `num as f64 / denom as f64`), daher liefert ein manipuliertes
/// oder beschaedigtes GPS-Rational mit Nenner 0 NaN oder Infinity. Solche
/// Werte werden hier verworfen, damit sie nicht weiter in ExifGps, die
/// Nominatim-URL oder die Datenbank gelangen.
pub fn parse_gps_coord(field: &exif::Field, ref_str: &str) -> Option<f64> {
    match &field.value {
        exif::Value::Rational(vec) if vec.len() >= 3 => {
            let deg = vec[0].to_f64();
            let min = vec[1].to_f64();
            let sec = vec[2].to_f64();
            if !deg.is_finite() || !min.is_finite() || !sec.is_finite() {
                return None;
            }
            let dec = dms_to_decimal(deg, min, sec, ref_str);
            if !dec.is_finite() {
                return None;
            }
            Some(dec)
        }
        _ => None,
    }
}

/// Prueft, ob ein Koordinatenpaar endlich und im gueltigen Wertebereich liegt
/// (Breite [-90, 90], Laenge [-180, 180]).
fn is_valid_coord(lat: f64, lon: f64) -> bool {
    lat.is_finite() && lon.is_finite() && (-90.0..=90.0).contains(&lat) && (-180.0..=180.0).contains(&lon)
}

/// Entfernt GPS-Daten aus den EXIF-Informationen (DSGVO Opt-out).
pub fn strip_gps(data: &mut ExifData) {
    data.gps = None;
}

/// Liest saemtliche relevanten EXIF-Felder aus dem Bild-Byte-Puffer.
pub fn extract_exif(bytes: &[u8]) -> Option<ExifData> {
    let mut cursor = Cursor::new(bytes);
    let exif = exif::Reader::new().read_from_container(&mut cursor).ok()?;

    let mut data = ExifData::default();

    let mut lat_field: Option<exif::Field> = None;
    let mut lat_ref = "N".to_string();
    let mut lon_field: Option<exif::Field> = None;
    let mut lon_ref = "E".to_string();
    let mut alt_field: Option<exif::Field> = None;
    let mut alt_ref: u8 = 0; // 0 = ueber dem Meeresspiegel, 1 = unter

    for field in exif.fields() {
        let tag = field.tag;
        let display = field.display_value().to_string();
        let clean = display.trim_matches('"').trim().to_string();

        match tag {
            exif::Tag::Make => data.make = Some(clean),
            exif::Tag::Model => data.model = Some(clean),
            exif::Tag::LensModel => data.lens_model = Some(clean),
            exif::Tag::ExposureTime => {
                let val = clean.trim_end_matches(" s").trim().to_string();
                data.exposure_time = Some(val);
            }
            exif::Tag::FNumber => {
                let val = if clean.starts_with("f/") {
                    clean
                } else {
                    format!("f/{clean}")
                };
                data.f_number = Some(val);
            }
            exif::Tag::PhotographicSensitivity | exif::Tag::ISOSpeed => {
                if let Ok(iso_val) = clean.parse::<u32>() {
                    data.iso = Some(iso_val);
                }
            }
            exif::Tag::FocalLength => {
                let val = if clean.ends_with("mm") {
                    clean
                } else if let Some(stripped) = clean.strip_suffix(" mm") {
                    format!("{stripped}mm")
                } else {
                    format!("{clean}mm")
                };
                data.focal_length = Some(val);
            }
            exif::Tag::DateTimeOriginal | exif::Tag::DateTime => {
                if data.datetime_original.is_none() {
                    if let Ok(naive) =
                        NaiveDateTime::parse_from_str(&clean, "%Y:%m:%d %H:%M:%S")
                    {
                        data.datetime_original =
                            Some(naive.and_utc().to_rfc3339());
                    } else {
                        data.datetime_original = Some(clean);
                    }
                }
            }
            exif::Tag::Orientation => {
                if let exif::Value::Short(ref v) = field.value {
                    if let Some(&first) = v.first() {
                        data.orientation = Some(first as u32);
                    }
                } else if let Ok(o) = clean.parse::<u32>() {
                    data.orientation = Some(o);
                }
            }
            exif::Tag::GPSLatitude => lat_field = Some(field.clone()),
            exif::Tag::GPSLatitudeRef => lat_ref = clean,
            exif::Tag::GPSLongitude => lon_field = Some(field.clone()),
            exif::Tag::GPSLongitudeRef => lon_ref = clean,
            exif::Tag::GPSAltitude => alt_field = Some(field.clone()),
            exif::Tag::GPSAltitudeRef => {
                if let exif::Value::Byte(ref v) = field.value {
                    if let Some(&b) = v.first() {
                        alt_ref = b;
                    }
                } else if let Ok(b) = clean.parse::<u8>() {
                    alt_ref = b;
                }
            }
            _ => {}
        }
    }

    // GPS-Koordinaten berechnen, falls vorhanden
    if let (Some(lat_f), Some(lon_f)) = (lat_field, lon_field) {
        if let (Some(lat), Some(lon)) = (
            parse_gps_coord(&lat_f, &lat_ref),
            parse_gps_coord(&lon_f, &lon_ref),
        ) {
            // 4 Nachkommastellen runden fuer einheitliche Praezision (~11 Meter)
            let lat_rounded = (lat * 10000.0).round() / 10000.0;
            let lon_rounded = (lon * 10000.0).round() / 10000.0;

            if is_valid_coord(lat_rounded, lon_rounded) {
                let altitude = alt_field.and_then(|f| match &f.value {
                    exif::Value::Rational(vec) if !vec.is_empty() => {
                        let mut alt = vec[0].to_f64();
                        if alt_ref == 1 {
                            alt = -alt;
                        }
                        if !alt.is_finite() {
                            return None;
                        }
                        Some((alt * 10.0).round() / 10.0)
                    }
                    _ => None,
                });

                data.gps = Some(ExifGps {
                    lat: lat_rounded,
                    lon: lon_rounded,
                    altitude,
                });
            } else {
                tracing::warn!(
                    lat = lat_rounded,
                    lon = lon_rounded,
                    "ungueltige GPS-Koordinate in EXIF verworfen"
                );
            }
        }
    }

    Some(data)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_dms_to_decimal() {
        // Zuerich: 47 Grad, 22 Minuten, 36.84 Sekunden N
        let lat = dms_to_decimal(47.0, 22.0, 36.84, "N");
        assert!((lat - 47.3769).abs() < 0.001);

        // 8 Grad, 32 Minuten, 30.12 Sekunden E
        let lon = dms_to_decimal(8.0, 32.0, 30.12, "E");
        assert!((lon - 8.5417).abs() < 0.001);

        // Suedliche Hemisphaere -> negativ
        let s_lat = dms_to_decimal(33.0, 51.0, 0.0, "S");
        assert!(s_lat < 0.0);
        assert!((s_lat - (-33.85)).abs() < 0.01);

        // Westliche Hemisphaere -> negativ
        let w_lon = dms_to_decimal(122.0, 25.0, 0.0, "W");
        assert!(w_lon < 0.0);
        assert!((w_lon - (-122.4167)).abs() < 0.01);
    }

    #[test]
    fn test_parse_gps_coord_zero_denominator_yields_none() {
        // kamadak-exif's Rational::to_f64 hat keine Nenner-Null-Pruefung und
        // liefert bei denom = 0 NaN/Infinity. parse_gps_coord muss das abfangen.
        let field = exif::Field {
            tag: exif::Tag::GPSLatitude,
            ifd_num: exif::In::PRIMARY,
            value: exif::Value::Rational(vec![
                exif::Rational { num: 47, denom: 1 },
                exif::Rational { num: 22, denom: 1 },
                exif::Rational { num: 1, denom: 0 },
            ]),
        };

        assert_eq!(parse_gps_coord(&field, "N"), None);
    }

    #[test]
    fn test_is_valid_coord_rejects_out_of_range() {
        // Gueltige Koordinaten (Zuerich)
        assert!(is_valid_coord(47.3769, 8.5417));

        // Ausserhalb des gueltigen Wertebereichs
        assert!(!is_valid_coord(91.0, 8.5417));
        assert!(!is_valid_coord(47.3769, 181.0));
        assert!(!is_valid_coord(-91.0, -8.5417));

        // Nicht endliche Werte (z.B. aus einem Nenner-Null-Rational)
        assert!(!is_valid_coord(f64::NAN, 8.5417));
        assert!(!is_valid_coord(47.3769, f64::INFINITY));
    }

    #[test]
    fn test_strip_gps() {
        let mut data = ExifData {
            make: Some("Apple".to_string()),
            gps: Some(ExifGps {
                lat: 47.3769,
                lon: 8.5417,
                altitude: Some(408.0),
            }),
            ..Default::default()
        };

        strip_gps(&mut data);
        assert!(data.gps.is_none());
        assert_eq!(data.make.as_deref(), Some("Apple"));
    }
}
