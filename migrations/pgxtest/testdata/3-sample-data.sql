--- !Up
insert into samples (name)
values ('abc');

insert into samples (name, email)
values ('zzz', '123@nowhere.com');

--- !Down
delete
from samples
where email is not null;

delete
from samples
where name = 'abc';
