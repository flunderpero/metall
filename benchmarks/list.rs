use std::env;

fn hash(mut x: u64) -> u64 {
    x ^= x >> 30;
    x = x.wrapping_mul(0xbf58476d1ce4e5b9);
    x ^= x >> 27;
    x = x.wrapping_mul(0x94d049bb133111eb);
    x ^= x >> 31;
    x
}

fn n() -> u64 {
    // Read N opaquely so the optimizer cannot precompute the whole run.
    env::var("LIST_N").ok().and_then(|s| s.parse().ok()).unwrap_or(10_000_000)
}

fn main() {
    let mode = env::args().nth(1).unwrap_or_else(|| "grow".to_string());
    let big_n = n();

    match mode.as_str() {
        "grow" => {
            let rounds = 100;
            let mut acc: u64 = 0;
            for _ in 0..rounds {
                let mut a: Vec<u64> = Vec::new();
                for i in 0..big_n {
                    let v = hash(i);
                    a.push(v);
                    acc = acc.wrapping_add(v);
                }
            }
            println!("grow: {}", acc);
        }
        "seq" => {
            let passes = 1000;
            let mut a: Vec<u64> = Vec::with_capacity(big_n as usize);
            for i in 0..big_n {
                a.push(hash(i));
            }
            let mut acc: u64 = 0;
            for _ in 0..passes {
                for &x in &a {
                    acc = acc.wrapping_add(x);
                }
            }
            println!("seq: {}", acc);
        }
        "random" => {
            let ops = big_n * 24;
            let mut a: Vec<u64> = Vec::with_capacity(big_n as usize);
            for i in 0..big_n {
                a.push(hash(i));
            }
            let mut acc: u64 = 0;
            for i in 0..ops {
                let idx = (hash(i) % big_n) as usize;
                acc = acc.wrapping_add(a[idx]);
            }
            println!("random: {}", acc);
        }
        "scatter" => {
            let ops = big_n * 23;
            let mut a: Vec<u64> = Vec::with_capacity(big_n as usize);
            for i in 0..big_n {
                a.push(hash(i));
            }
            for i in 0..ops {
                let idx = (hash(i) % big_n) as usize;
                a[idx] = a[idx].wrapping_add(hash(i));
            }
            let mut acc: u64 = 0;
            for &x in &a {
                acc = acc.wrapping_add(x);
            }
            println!("scatter: {}", acc);
        }
        "drain" => {
            let rounds = 200;
            let mut a: Vec<u64> = Vec::with_capacity(big_n as usize);
            let mut acc: u64 = 0;
            for _ in 0..rounds {
                a.clear();
                for i in 0..big_n {
                    a.push(hash(i));
                }
                while let Some(v) = a.pop() {
                    acc = acc.wrapping_add(v);
                }
            }
            println!("drain: {}", acc);
        }
        other => {
            eprintln!("Unknown mode: {}", other);
            std::process::exit(1);
        }
    }
}
