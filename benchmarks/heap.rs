use std::collections::BinaryHeap;
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
    env::var("HEAP_N").ok().and_then(|s| s.parse().ok()).unwrap_or(10_000_000)
}

fn main() {
    let mode = env::args().nth(1).unwrap_or_else(|| "sort".to_string());
    let big_n = n();
    const K: u64 = 1024;

    match mode.as_str() {
        "sort" => {
            // BinaryHeap is a max-heap; pop() yields the largest element.
            let mut h: BinaryHeap<u64> = BinaryHeap::with_capacity(big_n as usize);
            for i in 0..big_n {
                h.push(hash(i));
            }
            let mut acc: u64 = 0;
            while let Some(v) = h.pop() {
                acc = acc.wrapping_mul(1000003).wrapping_add(v);
            }
            println!("sort: {}", acc);
        }
        "churn" => {
            let m = big_n * 3;
            let mut h: BinaryHeap<u64> = BinaryHeap::with_capacity((K + 1) as usize);
            let mut acc: u64 = 0;
            for i in 0..m {
                h.push(hash(i));
                if h.len() as u64 > K {
                    acc = acc.wrapping_mul(1000003).wrapping_add(h.pop().unwrap());
                }
            }
            while let Some(v) = h.pop() {
                acc = acc.wrapping_mul(1000003).wrapping_add(v);
            }
            println!("churn: {}", acc);
        }
        "pushpop" => {
            let m = big_n * 3;
            let mut h: BinaryHeap<u64> = BinaryHeap::with_capacity(K as usize);
            for i in 0..K {
                h.push(hash(i));
            }
            let mut acc: u64 = 0;
            for i in 0..m {
                let top = h.pop().unwrap();
                acc = acc.wrapping_mul(1000003).wrapping_add(top);
                h.push(hash(K + i));
            }
            while let Some(v) = h.pop() {
                acc = acc.wrapping_mul(1000003).wrapping_add(v);
            }
            println!("pushpop: {}", acc);
        }
        other => {
            eprintln!("Unknown mode: {}", other);
            std::process::exit(1);
        }
    }
}
