export LD_LIBRARY_PATH=`realpath ../build/install/server`:`realpath ../`:$LD_LIBRARY_PATH
node ./dist/bin/main.js --launcher ../build/install/server/Renderer_Win --port 43210 $@
