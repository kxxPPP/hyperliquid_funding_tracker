platforms=("windows/amd64" "darwin/arm64" "linux/amd64")

for platform in "${platforms[@]}"
do

    os=${platform%/*}   
    arch=${platform#*/}  

    if [ "$os" == "windows" ]; then
        output="gh-windows-$arch.exe"  
    else
        output="gh-$os-$arch"
    fi

    env GOOS=$os GOARCH=$arch go build -o $output

    echo "Built for $os/$arch: $output"
done
