use std::env;

fn hash(mut x: u64) -> u64 {
    x ^= x >> 30;
    x = x.wrapping_mul(0xbf58476d1ce4e5b9);
    x ^= x >> 27;
    x = x.wrapping_mul(0x94d049bb133111eb);
    x ^= x >> 31;
    x
}

#[inline]
fn is_space(b: u8) -> bool {
    b == b' ' || (9..=13).contains(&b)
}

fn main() {
    let mode = env::args().nth(1).unwrap_or_else(|| "byte".to_string());
    let n: usize = 500_000_000;
    let mut data: Vec<u8> = vec![0u8; n];
    for i in 0..n {
        data[i] = (hash(i as u64) % 256) as u8;
    }
    let needle: u8 = 7;
    let r = match mode.as_str() {
        "byte" => data.split(|&b| b == needle).count(),
        "predicate" => data.split(|&b| is_space(b)).count(),
        _ => {
            eprintln!("unknown mode: {}", mode);
            std::process::exit(1);
        }
    };
    println!("{}", r);
}
