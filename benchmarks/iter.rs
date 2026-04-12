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
    // We read N from the environment so LLVM cannot constant fold parts of the `hash`
    // function. If N was constant, LLVM could "unfairly" optimize the iteration source
    // and that is not what we want to test.
    env::var("ITER_N").ok().and_then(|s| s.parse().ok()).unwrap_or(500_000_000)
}

fn pipeline() -> impl Iterator<Item = u64> {
    (0..n())
        .map(|x| hash(x))
        .filter(|&x| x % 3 != 0)
        .map(|x| x.wrapping_mul(17).wrapping_add(42))
}

fn main() {
    let mode = env::args().nth(1).unwrap_or_else(|| "fold".to_string());
    match mode.as_str() {
        "fold" => {
            let r = pipeline().fold(0u64, |acc, x| acc.wrapping_add(x));
            println!("fold: {}", r);
        }
        "count" => {
            let r = pipeline().count();
            println!("count: {}", r);
        }
        "all" => {
            let r = pipeline().all(|x| x < u64::MAX);
            println!("all: {}", r);
        }
        "any" => {
            let target = hash(499_999_990).wrapping_mul(17).wrapping_add(42);
            let r = pipeline().any(|x| x == target);
            println!("any: {}", r);
        }
        "find" => {
            let target = hash(499_999_990).wrapping_mul(17).wrapping_add(42);
            match pipeline().find(|&x| x == target) {
                Some(v) => println!("find: {}", v),
                None => println!("find: None"),
            }
        }
        "collect" => {
            let r: Vec<u64> = pipeline().collect();
            println!("collect: {}", r.len());
        }
        "take" => {
            let r = pipeline().take(100_000_000).fold(0u64, |acc, x| acc.wrapping_add(x));
            println!("take: {}", r);
        }
        "take_while" => {
            let threshold = hash(499_999_990).wrapping_mul(17).wrapping_add(42);
            let r = pipeline().take_while(|&x| x != threshold)
                .fold(0u64, |acc, x| acc.wrapping_add(x));
            println!("take_while: {}", r);
        }
        other => {
            eprintln!("Unknown mode: {}", other);
            std::process::exit(1);
        }
    }
}
