# Stitchy

## DB Table

    create table photos (
        id serial primary key,
        user_id varchar(255) not null,
        photo_url varchar(255) not null,
        created_at timestamp default now()
    );
