# Stitchy

## Running

Need the environment variables:

- AWS_ACCESS_KEY_ID
- AWS_SECRET_ACCESS_KEY

## DB Table

    create table photos (
        id serial primary key,
        user_id varchar(255) not null,
        photo_url varchar(255) not null,
        created_at timestamp default now()
    );

## API

   - POST /v1/users/:user_id/photo ?url=
   - POST /v1/users/:user_id/stitch
   - GET /v1/users/:user_id/stitch
     - Return { } - No Job
     - Return { "status": "in-progress", "" }
     - Return { "video_url": "" }
