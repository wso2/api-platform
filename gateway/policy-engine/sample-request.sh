curl 'http://localhost:8000/pets/myPetId123/history?bar=baz&param_to_remove=bbbbb' \
-iv \
-d '{
   "name": "John Doe",
   "age": 30,
   "address": "123 Main St, San Francisco, CA 94123",
   "phone": "123-456-7890",
   "email": "john@abc.com"
}' \
-H 'foo: hello-foo1' \
-H 'foo: hello-foo2' \
-H 'x-internal-token: my-password' \
-u 'admin:secret123' 
# -H 'count: true' \