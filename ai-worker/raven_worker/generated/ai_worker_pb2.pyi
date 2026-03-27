from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ParseRequest(_message.Message):
    __slots__ = ("document_id", "org_id", "kb_id", "content", "mime_type", "file_name")
    DOCUMENT_ID_FIELD_NUMBER: _ClassVar[int]
    ORG_ID_FIELD_NUMBER: _ClassVar[int]
    KB_ID_FIELD_NUMBER: _ClassVar[int]
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    MIME_TYPE_FIELD_NUMBER: _ClassVar[int]
    FILE_NAME_FIELD_NUMBER: _ClassVar[int]
    document_id: str
    org_id: str
    kb_id: str
    content: bytes
    mime_type: str
    file_name: str
    def __init__(self, document_id: _Optional[str] = ..., org_id: _Optional[str] = ..., kb_id: _Optional[str] = ..., content: _Optional[bytes] = ..., mime_type: _Optional[str] = ..., file_name: _Optional[str] = ...) -> None: ...

class ParseResponse(_message.Message):
    __slots__ = ("document_id", "chunk_count", "status", "error_message")
    DOCUMENT_ID_FIELD_NUMBER: _ClassVar[int]
    CHUNK_COUNT_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    ERROR_MESSAGE_FIELD_NUMBER: _ClassVar[int]
    document_id: str
    chunk_count: int
    status: str
    error_message: str
    def __init__(self, document_id: _Optional[str] = ..., chunk_count: _Optional[int] = ..., status: _Optional[str] = ..., error_message: _Optional[str] = ...) -> None: ...

class RAGRequest(_message.Message):
    __slots__ = ("query", "org_id", "kb_ids", "session_id", "filters", "model", "provider")
    class FiltersEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    QUERY_FIELD_NUMBER: _ClassVar[int]
    ORG_ID_FIELD_NUMBER: _ClassVar[int]
    KB_IDS_FIELD_NUMBER: _ClassVar[int]
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    FILTERS_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    query: str
    org_id: str
    kb_ids: _containers.RepeatedScalarFieldContainer[str]
    session_id: str
    filters: _containers.ScalarMap[str, str]
    model: str
    provider: str
    def __init__(self, query: _Optional[str] = ..., org_id: _Optional[str] = ..., kb_ids: _Optional[_Iterable[str]] = ..., session_id: _Optional[str] = ..., filters: _Optional[_Mapping[str, str]] = ..., model: _Optional[str] = ..., provider: _Optional[str] = ...) -> None: ...

class RAGChunk(_message.Message):
    __slots__ = ("text", "is_final", "sources")
    TEXT_FIELD_NUMBER: _ClassVar[int]
    IS_FINAL_FIELD_NUMBER: _ClassVar[int]
    SOURCES_FIELD_NUMBER: _ClassVar[int]
    text: str
    is_final: bool
    sources: _containers.RepeatedCompositeFieldContainer[Source]
    def __init__(self, text: _Optional[str] = ..., is_final: bool = ..., sources: _Optional[_Iterable[_Union[Source, _Mapping]]] = ...) -> None: ...

class Source(_message.Message):
    __slots__ = ("document_id", "document_name", "chunk_text", "score")
    DOCUMENT_ID_FIELD_NUMBER: _ClassVar[int]
    DOCUMENT_NAME_FIELD_NUMBER: _ClassVar[int]
    CHUNK_TEXT_FIELD_NUMBER: _ClassVar[int]
    SCORE_FIELD_NUMBER: _ClassVar[int]
    document_id: str
    document_name: str
    chunk_text: str
    score: float
    def __init__(self, document_id: _Optional[str] = ..., document_name: _Optional[str] = ..., chunk_text: _Optional[str] = ..., score: _Optional[float] = ...) -> None: ...

class EmbeddingRequest(_message.Message):
    __slots__ = ("text", "org_id", "model", "provider")
    TEXT_FIELD_NUMBER: _ClassVar[int]
    ORG_ID_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    text: str
    org_id: str
    model: str
    provider: str
    def __init__(self, text: _Optional[str] = ..., org_id: _Optional[str] = ..., model: _Optional[str] = ..., provider: _Optional[str] = ...) -> None: ...

class EmbeddingResponse(_message.Message):
    __slots__ = ("embedding", "dimensions")
    EMBEDDING_FIELD_NUMBER: _ClassVar[int]
    DIMENSIONS_FIELD_NUMBER: _ClassVar[int]
    embedding: _containers.RepeatedScalarFieldContainer[float]
    dimensions: int
    def __init__(self, embedding: _Optional[_Iterable[float]] = ..., dimensions: _Optional[int] = ...) -> None: ...
