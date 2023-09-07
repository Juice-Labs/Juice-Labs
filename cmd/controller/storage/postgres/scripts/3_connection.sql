alter table connections
    drop column exit_status,
    add column exit_code int;

drop type connection_exit_status;
