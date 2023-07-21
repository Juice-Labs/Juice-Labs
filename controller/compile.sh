if [ "$(id -u)" -eq 0 ]; then
    npm install --unsafe-perm
else
    npm install
fi
./node_modules/.bin/tsc --build --clean
./node_modules/.bin/tsc
