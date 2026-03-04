use std::io::Result;

fn main() -> Result<()> {
    let proto_dir = "../../proto";
    prost_build::compile_protos(
        &[
            format!("{proto_dir}/Blobs.proto"),
            format!("{proto_dir}/Execution.proto"),
            format!("{proto_dir}/Local.proto"),
            format!("{proto_dir}/Contester.proto"),
        ],
        &[proto_dir],
    )?;
    Ok(())
}
