#!/usr/bin/env python3
"""Build the checked-in OpenAPI contract. Run after intentional API changes."""
import json
from pathlib import Path


def ref(name):
    return {'$ref': '#/components/schemas/' + name}


def string(**kw):
    return {'type': 'string', **kw}


def integer(**kw):
    return {'type': 'integer', **kw}


def obj(properties, required=None, **kw):
    return {'type': 'object', 'additionalProperties': False, 'properties': properties,
            'required': list(properties) if required is None else required, **kw}


uid = string(format='uuid')
time = string(format='date-time')
name = string(minLength=1, maxLength=100)
email = string(format='email', maxLength=254, description='Trimmed and converted to lowercase.')
password = string(minLength=12, maxLength=128, format='password', writeOnly=True,
                  description='At least 12 Unicode characters, at most 128 UTF-8 bytes.')
birth = integer(minimum=1900, maximum=2200, description='Must not exceed the current UTC year.')
year = integer(minimum=1, maximum=2200, description='Must not exceed next UTC year.')
volume = integer(minimum=1, maximum=2147483647, nullable=True)
profile = {'first_name': name, 'last_name': name, 'middle_name': string(maxLength=100, nullable=True),
           'birth_year': birth, 'email': email}
book = {'title': string(minLength=1, maxLength=300, description='Trimmed, nonblank.'),
        'author': string(minLength=1, maxLength=200, description='Trimmed, nonblank.'),
        'publication_year': year, 'volume': volume}
schemas = {
    'Register': obj({**profile, 'password': password}, ['first_name', 'last_name', 'birth_year', 'email', 'password']),
    'Login': obj({'email': email, 'password': string(minLength=1, maxLength=128, format='password', writeOnly=True)}),
    'Refresh': obj({'refresh_token': string(minLength=43, maxLength=43, writeOnly=True, description='Opaque refresh credential returned by login or rotation.')}),
    'ProfilePatch': obj({k: {**v, 'nullable': True} for k, v in profile.items()}, [], description='Omitted or null fields are unchanged. Empty middle_name clears the displayed name. Empty object is a no-op. Role/status/password are never accepted.'),
    'PasswordChange': obj({'current_password': string(minLength=1, maxLength=128, format='password', writeOnly=True), 'new_password': password}),
    'BookCreate': obj(book, ['title', 'author', 'publication_year']),
    'BookPatch': obj({k: {**v, 'nullable': True} for k, v in book.items()}, [], description='Omitted or null fields are unchanged. Empty object is a no-op. Counters are server controlled.'),
    'CommentInput': obj({'content': string(minLength=1, maxLength=4000, description='Trimmed and must not be blank. Identity comes only from the authenticated account.')}),
    'User': obj({'id': uid, **profile, 'role': string(enum=['ADMIN', 'READER']), 'status': string(enum=['ACTIVE', 'BLOCKED']), 'created_at': time, 'updated_at': time}),
    'Book': obj({'id': uid, **book, 'likes_count': integer(format='int64', minimum=0), 'comments_count': integer(format='int64', minimum=0), 'created_at': time, 'updated_at': time}),
    'ReaderSummary': obj({'id': uid, 'first_name': string(), 'last_name': string()}),
    'Comment': obj({'id': uid, 'book_id': uid, 'content': string(), 'reader': ref('ReaderSummary'), 'created_at': time, 'updated_at': time}),
    'Credentials': obj({'access_token': string(), 'refresh_token': string(), 'token_type': string(enum=['Bearer']), 'expires_in': integer(description='Access token lifetime in seconds.'), 'user': ref('User')}),
    'Pagination': obj({'page': integer(minimum=1, maximum=1000000), 'page_size': integer(minimum=1, maximum=100), 'total': integer(format='int64', minimum=0), 'total_pages': integer(format='int64', minimum=0)}),
    'Error': obj({'error': obj({'code': string(), 'message': string(), 'fields': {'type': 'object', 'additionalProperties': string()}}, ['code', 'message'])}),
    'Health': obj({'data': obj({'status': string(enum=['alive', 'ready'])})}),
}
for resource in ['User', 'Book', 'Comment', 'Credentials']:
    schemas[resource + 'Envelope'] = obj({'data': ref(resource)})
for resource in ['User', 'Book', 'Comment']:
    schemas[resource + 'Collection'] = obj({'data': {'type': 'array', 'items': ref(resource)}, 'pagination': ref('Pagination')})

errors = {str(code): {'description': desc, 'content': {'application/json': {'schema': ref('Error')}}} for code, desc in {
    400: 'Malformed JSON, unknown request field, or multiple JSON values.',
    401: 'Authentication missing/invalid; login failures do not reveal blocked or nonexistent accounts.',
    403: 'Wrong role, ownership denied, disallowed origin, or blocked account.',
    404: 'Resource not found.', 409: 'Duplicate email or relational conflict.',
    413: 'Request body exceeds 1 MiB.', 415: 'Content-Type must be application/json.',
    422: 'Field validation or query parameter failure.', 429: 'Authentication work limit reached; Retry-After: 1.',
    500: 'Unexpected error; internal details are never exposed.', 504: 'Request deadline exceeded.',
    503: 'Database is unavailable; readiness is false.',
}.items()}
paths = {}


def param(name, schema, description=''):
    return {'name': name, 'in': 'query', 'required': False, 'schema': schema, 'description': description}


def pagination(sorts):
    return [param('page', integer(default=1, minimum=1, maximum=1000000)),
            param('page_size', integer(default=20, minimum=1, maximum=100)),
            param('sort', string(enum=sorts, default='created_at')),
            param('order', string(enum=['asc', 'desc'], default='desc'))]


def add(path, method, operation, summary, tag, role='ADMIN or READER', request=None, response=None, status=200,
        query=None, description='', extra=()):
    responses = {str(status): {'description': 'Success'}}
    if response:
        responses[str(status)]['content'] = {'application/json': {'schema': ref(response)}}
    codes = [400, 403, 422, 500, 504] + list(extra)
    if role != 'Public':
        codes += [401]
    if '{' in path:
        codes += [404]
    if request:
        codes += [413, 415]
    responses.update({str(c): errors[str(c)] for c in sorted(set(codes))})
    op = {'operationId': operation, 'summary': summary, 'tags': [tag],
          'description': f'Role: {role}. ' + description, 'responses': responses}
    if role == 'Public':
        op['security'] = []
    parameters = list(query or [])
    for p in path.split('/'):
        if p.startswith('{'):
            parameters.append({'name': p[1:-1], 'in': 'path', 'required': True, 'schema': uid})
    if parameters:
        op['parameters'] = parameters
    if request:
        op['requestBody'] = {'required': True, 'content': {'application/json': {'schema': ref(request)}}}
    paths.setdefault(path, {})[method] = op


base = '/api/v1'
add(base+'/auth/register', 'post', 'register', 'Register a READER', 'Authentication', 'Public', 'Register', 'UserEnvelope', 201, extra=[409, 429])
add(base+'/auth/login', 'post', 'login', 'Log in with email and password', 'Authentication', 'Public', 'Login', 'CredentialsEnvelope', extra=[401, 429])
add(base+'/auth/refresh', 'post', 'refresh', 'Rotate a refresh credential', 'Authentication', 'Public', 'Refresh', 'CredentialsEnvelope', extra=[401], description='Previous credential becomes unusable. Reuse revokes the entire family; concurrent refresh requests must be serialized by the client. Family expiry is absolute.')
add(base+'/auth/logout', 'post', 'logout', 'Revoke a refresh family', 'Authentication', 'Public', 'Refresh', status=204, description='Idempotent for validly shaped unknown/revoked credentials. Access JWTs remain usable until their short expiry.', extra=[401])
add(base+'/me', 'get', 'me', 'Get current profile', 'Profile', response='UserEnvelope')
add(base+'/me', 'patch', 'patchMe', 'Update current profile', 'Profile', request='ProfilePatch', response='UserEnvelope', extra=[409])
add(base+'/me/password', 'patch', 'changePassword', 'Change password and revoke credentials', 'Profile', request='PasswordChange', status=204, extra=[429], description='Validates current password, increments credential version and revokes all refresh families. Log in again afterwards.')
reader_path = base+'/admin/readers'
add(reader_path, 'get', 'listReaders', 'List readers', 'Readers', 'ADMIN', response='UserCollection', query=pagination(['created_at', 'first_name', 'last_name', 'email', 'birth_year'])+[param('status', string(enum=['ACTIVE','BLOCKED'])),param('first_name',string(maxLength=100)),param('last_name',string(maxLength=100)),param('email',string(maxLength=254))])
add(reader_path+'/{id}', 'get', 'getReader', 'Get a reader', 'Readers', 'ADMIN', response='UserEnvelope')
add(reader_path+'/{id}', 'patch', 'patchReader', 'Update a reader profile', 'Readers', 'ADMIN', 'ProfilePatch', 'UserEnvelope', extra=[409])
add(reader_path+'/{id}', 'delete', 'deleteReader', 'Soft-delete a reader', 'Readers', 'ADMIN', status=204, description='Revokes authentication; email stays reserved and existing likes/comments are retained. Comment attribution displays Deleted reader.')
for action in ['block','unblock']:
    add(reader_path+'/{id}/'+action, 'patch', action+'Reader', action.title()+' a reader', 'Readers', 'ADMIN', status=204, description='Account status changes invalidate existing credentials. Reader must log in again after unblocking.')
book_sorts=['created_at','title','author','publication_year','likes_count','comments_count']
book_path=base+'/books'
add(book_path, 'get', 'listBooks', 'List books', 'Books', response='BookCollection', query=pagination(book_sorts)+[param('title',string(maxLength=300)),param('author',string(maxLength=200)),param('publication_year',year)])
add(book_path, 'post', 'createBook', 'Create a book', 'Books', 'ADMIN', 'BookCreate', 'BookEnvelope', 201)
add(book_path+'/{id}', 'get', 'getBook', 'Get a book', 'Books', response='BookEnvelope')
add(book_path+'/{id}', 'patch', 'patchBook', 'Update a book', 'Books', 'ADMIN', 'BookPatch', 'BookEnvelope')
add(book_path+'/{id}', 'delete', 'deleteBook', 'Delete a book and its interactions', 'Books', 'ADMIN', status=204)
for action in ['favorite','like']:
    for method,verb in [('post','Add'),('delete','Remove')]:
        add(book_path+'/{id}/'+action, method, verb.lower()+action.title(), verb+' '+action, 'Interactions', 'READER', status=204, description='Idempotent. Returns 204 when the desired state already exists. Missing books return 404.')
add(base+'/me/favorites', 'get', 'listFavorites', 'List favorite books', 'Interactions', 'READER', response='BookCollection', query=pagination(book_sorts))
comments=book_path+'/{id}/comments'
add(comments, 'get', 'listComments', 'List book comments', 'Comments', response='CommentCollection', query=pagination(['created_at','updated_at']))
add(comments, 'post', 'createComment', 'Create a comment', 'Comments', 'READER', 'CommentInput', 'CommentEnvelope', 201)
add(comments+'/{commentId}', 'patch', 'patchComment', 'Edit an owned comment', 'Comments', 'READER (owner)', 'CommentInput', 'CommentEnvelope')
add(comments+'/{commentId}', 'delete', 'deleteComment', 'Delete a comment', 'Comments', 'ADMIN or READER (owner)', status=204)
add('/health/live', 'get', 'liveness', 'Process liveness', 'Operations', 'Public', response='Health')
add('/health/ready', 'get', 'readiness', 'PostgreSQL readiness', 'Operations', 'Public', response='Health', extra=[503])
add('/metrics', 'get', 'metrics', 'Prometheus metrics', 'Operations', 'ADMIN')
paths['/metrics']['get']['responses']['200']['content']={'text/plain':{'schema':string()}}
spec={'openapi':'3.0.3','info':{'title':'Library Service API','version':'1.0.0','description':'Strict JSON; versioned REST; PostgreSQL-backed authorization and atomic interactions. Collection search uses case-insensitive literal substrings. JSON null in PATCH means unchanged. Unknown request fields are rejected.'},
      'servers':[{'url':'/'}], 'security':[{'bearerAuth':[]}],
      'tags':[{'name':s} for s in ['Authentication','Profile','Readers','Books','Interactions','Comments','Operations']],
      'paths':paths,'components':{'schemas':schemas,'securitySchemes':{'bearerAuth':{'type':'http','scheme':'bearer','bearerFormat':'JWT'}}}}
Path(__file__).resolve().parents[1].joinpath('docs/openapi.json').write_text(json.dumps(spec,indent=2)+'\n')
