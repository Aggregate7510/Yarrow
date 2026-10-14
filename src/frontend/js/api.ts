import type {
  Feed,
  Folder,
  Item,
  Settings,
  StatusResponse,
  ItemListResponse,
  FeedCreateResponse,
  CrawlResponse,
  FeedCreateData,
  FeedUpdateData,
  FolderCreateData,
  FolderUpdateData,
  ItemUpdateData,
  ItemListQuery,
  SettingsUpdateData,
  ItemMarkQuery,
  CardListResponse,
} from "./api-types";

export class HTTPError extends Error {
  status: number;
  statusText: string;
  constructor(res: Response) {
    super(res.statusText);
    this.status = res.status;
    this.statusText = res.statusText;
  }
}

/** True when the response came from the offline cache of the service worker. */
export let lastResponseFromCache = false;

export class NetworkError extends Error {
  constructor(msg: string) {
    super(msg);
  }
}

type ApiOptions = {
  json?: object;
  body?: BodyInit;
  query?: Record<string, string | number | boolean | undefined>;
};

function api(method: string, endpoint: string, opts: ApiOptions = {}) {
  const { json, body, query } = opts;

  let url = endpoint;

  const init: RequestInit = {};
  init.method = method;
  init.headers = new Headers();
  init.headers.set("x-requested-by", "yarr");

  if (query !== undefined) {
    const clean: Record<string, string> = {};
    for (const [k, v] of Object.entries(query)) {
      if (v !== undefined && v !== null && v !== "") clean[k] = String(v);
    }
    url = url + "?" + new URLSearchParams(clean).toString();
  }
  if (body !== undefined) {
    init.body = body;
  }
  if (json !== undefined) {
    init.headers.set("content-type", "application/json");
    init.body = JSON.stringify(json);
  }

  return fetch(url, init)
    .catch(err => {
      if (err instanceof TypeError) throw new NetworkError(err.message);
      throw err;
    })
    .then(res => {
      if (method === "get") lastResponseFromCache = res.headers.get("x-yarrow-offline") === "1";
      if (!res.ok) throw new HTTPError(res);
      return res;
    });
}

function json<T>(res: Response): Promise<T> {
  return res.json() as Promise<T>;
}

export default {
  feeds: {
    list(): Promise<Feed[]> {
      return api("get", "./api/feeds").then(json<Feed[]>);
    },
    create(data: FeedCreateData): Promise<FeedCreateResponse> {
      return api("post", "./api/feeds", { json: data }).then(json<FeedCreateResponse>);
    },
    update(id: number, data: FeedUpdateData): Promise<Response> {
      return api("put", `./api/feeds/${id}`, { json: data });
    },
    delete(id: number): Promise<Response> {
      return api("delete", `./api/feeds/${id}`);
    },
    refresh(): Promise<Response> {
      return api("post", "./api/feeds/refresh");
    },
    list_errors(): Promise<Record<number, string>> {
      return api("get", "./api/feeds/errors").then(json<Record<number, string>>);
    },
  },
  folders: {
    list(): Promise<Folder[]> {
      return api("get", "./api/folders").then(json<Folder[]>);
    },
    create(data: FolderCreateData): Promise<Folder> {
      return api("post", "./api/folders", { json: data }).then(json<Folder>);
    },
    update(id: number, data: FolderUpdateData): Promise<Response> {
      return api("put", `./api/folders/${id}`, { json: data });
    },
    delete(id: number): Promise<Response> {
      return api("delete", `./api/folders/${id}`);
    },
  },
  items: {
    get(id: number): Promise<Item> {
      return api("get", `./api/items/${id}`).then(json<Item>);
    },
    list(query?: ItemListQuery): Promise<ItemListResponse> {
      // TODO: fix query annotation
      return api("get", "./api/items", { query }).then(
        json<ItemListResponse>,
      );
    },
    cards(query: ItemListQuery): Promise<CardListResponse> {
      return api("get", "./api/items", {
        query: { ...query, view: "cards" },
      }).then(json<CardListResponse>);
    },
    update(id: number, data: ItemUpdateData): Promise<Response> {
      return api("put", `./api/items/${id}`, { json: data });
    },
    mark_read(query: ItemMarkQuery): Promise<Response> {
      // TODO: fix query annotation
      return api("put", "./api/items", { query });
    },
  },
  settings: {
    get(): Promise<Settings> {
      return api("get", "./api/settings").then(json<Settings>);
    },
    update(data: SettingsUpdateData): Promise<Response> {
      return api("put", "./api/settings", { json: data });
    },
  },
  status(): Promise<StatusResponse> {
    return api("get", "./api/status").then(json<StatusResponse>);
  },
  upload_opml(form: HTMLFormElement): Promise<Response> {
    return api("post", "./opml/import", { body: new FormData(form) });
  },
  logout(): Promise<Response> {
    return api("post", "./logout");
  },
  crawl(url: string): Promise<CrawlResponse> {
    return api("get", "./page", { query: { url } }).then(json<CrawlResponse>);
  },
};
