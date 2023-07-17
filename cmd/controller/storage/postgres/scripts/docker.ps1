docker run -e POSTGRES_PASSWORD="password" -d -p 5432:5432 -v ${pwd}:/docker-entrypoint-initdb.d/ postgres:13
