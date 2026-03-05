//! runner — daemon that connects to a dispatcher and serves RPC requests.

use std::time::Duration;

use anyhow::Result;
use clap::Parser;
use tokio::net::TcpStream;
use tracing::{error, info};

use contester_rpc4::ServerCodec;
use contester_service::{Contester, ContesterConfig};

#[derive(Parser)]
#[command(name = "runner", version, about = "Contester runner daemon")]
struct Args {
    /// Log file path
    #[arg(short = 'l', long = "logfile", default_value = "server.log")]
    logfile: String,

    /// Config file path (TOML)
    #[arg(short = 'c', long = "config", default_value = "server.toml")]
    config_file: String,
}

#[tokio::main]
async fn main() -> Result<()> {
    let args = Args::parse();

    // Set up file logging
    let logfile = std::fs::OpenOptions::new()
        .create(true)
        .append(true)
        .open(&args.logfile)?;

    tracing_subscriber::fmt()
        .with_writer(move || logfile.try_clone().unwrap())
        .init();

    // Load config
    let config_str = std::fs::read_to_string(&args.config_file)?;
    let config: ContesterConfig = toml::from_str(&config_str)?;

    let server_address = config.server.clone();

    // Create service
    let contester = Contester::new(config)?;

    info!("Runner starting, connecting to {}", server_address);

    // Reconnect loop
    loop {
        match TcpStream::connect(&server_address).await {
            Ok(stream) => {
                info!("Connected to {}", server_address);
                let mut codec = ServerCodec::from_tcp(stream);

                // Serve requests until connection drops
                loop {
                    match codec.read_request().await {
                        Ok(request) => {
                            let method = request.method.clone();
                            info!("Handling {}", method);
                            if let Err(e) = contester.dispatch(&mut codec, request).await {
                                error!("Error handling {}: {}", method, e);
                            }
                        }
                        Err(e) => {
                            error!("Connection error: {}", e);
                            break;
                        }
                    }
                }
            }
            Err(e) => {
                error!("Connection failed: {}", e);
            }
        }

        info!("Reconnecting in 5 seconds...");
        tokio::time::sleep(Duration::from_secs(5)).await;
    }
}
