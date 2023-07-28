To initialize a new PSQL database, run all migrations in this folder in sequence.

For local development: 
1. Install docker
2. Run docker1.ps1 to start a local PSQL server in a docker container, note the <container id>
4. Run SQL script
    a. docker cp Juice-Labs\cmd\controller\storage\postgres\scripts\1_init.sql <container id>::/var/lib/postgresql/
    b. docker exec -it --user postgres <container id> psql -d postgres -a -f /var/lib/postgresql/1_init.sql
3. Connect to postgres via 
 docker exec -it --user postgres <container id> psql
4. Use the following connection string when running the controller:
host=localhost port=5432 user=postgres password=password dbname=postgres sslmode=disable

TODO: Script to initialize db