export namespace model {
	
	export class Detail {
	    id: string;
	    title: string;
	    slug: string;
	    path: string;
	    address: string;
	    box: string;
	    filed: boolean;
	    parent: string;
	    children: string[];
	    links: string[];
	    tags: string[];
	    // Go type: time
	    created: any;
	    guest: boolean;
	    damaged: boolean;
	    excerpt: string;
	    body: string;
	    // Go type: time
	    modTime: any;
	    links: slipbox.Link[];
	    backlinks: string[];
	    crossrefs: string[];
	    extra: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new Detail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.slug = source["slug"];
	        this.path = source["path"];
	        this.address = source["address"];
	        this.box = source["box"];
	        this.filed = source["filed"];
	        this.parent = source["parent"];
	        this.children = source["children"];
	        this.links = source["links"];
	        this.tags = source["tags"];
	        this.created = this.convertValues(source["created"], null);
	        this.guest = source["guest"];
	        this.damaged = source["damaged"];
	        this.excerpt = source["excerpt"];
	        this.body = source["body"];
	        this.modTime = this.convertValues(source["modTime"], null);
	        this.links = this.convertValues(source["links"], slipbox.Link);
	        this.backlinks = source["backlinks"];
	        this.crossrefs = source["crossrefs"];
	        this.extra = source["extra"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Note {
	    id: string;
	    title: string;
	    slug: string;
	    path: string;
	    address: string;
	    box: string;
	    filed: boolean;
	    parent: string;
	    children: string[];
	    links: string[];
	    tags: string[];
	    // Go type: time
	    created: any;
	    guest: boolean;
	    damaged: boolean;
	    excerpt: string;
	
	    static createFrom(source: any = {}) {
	        return new Note(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.slug = source["slug"];
	        this.path = source["path"];
	        this.address = source["address"];
	        this.box = source["box"];
	        this.filed = source["filed"];
	        this.parent = source["parent"];
	        this.children = source["children"];
	        this.links = source["links"];
	        this.tags = source["tags"];
	        this.created = this.convertValues(source["created"], null);
	        this.guest = source["guest"];
	        this.damaged = source["damaged"];
	        this.excerpt = source["excerpt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Vault {
	    root: string;
	    notes: Note[];
	    drawers: slipbox.Drawer[];
	    desk: slipbox.Placement[];
	    boards: slipbox.Board[];
	    settings: slipbox.Settings;
	    warnings: slipbox.Warning[];
	    stage: string;
	    rediscover: string;
	    unfiled: string[];
	    boxes: slipbox.Box[];
	    register: slipbox.RegisterEntry[];
	    prefs: Record<string, any>;
	    incomplete: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Vault(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root = source["root"];
	        this.notes = this.convertValues(source["notes"], Note);
	        this.drawers = this.convertValues(source["drawers"], slipbox.Drawer);
	        this.desk = this.convertValues(source["desk"], slipbox.Placement);
	        this.boards = this.convertValues(source["boards"], slipbox.Board);
	        this.settings = this.convertValues(source["settings"], slipbox.Settings);
	        this.warnings = this.convertValues(source["warnings"], slipbox.Warning);
	        this.stage = source["stage"];
	        this.rediscover = source["rediscover"];
	        this.unfiled = source["unfiled"];
	        this.boxes = this.convertValues(source["boxes"], slipbox.Box);
	        this.register = this.convertValues(source["register"], slipbox.RegisterEntry);
	        this.prefs = source["prefs"];
	        this.incomplete = source["incomplete"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace slipbox {
	
	export class Placement {
	    id: string;
	    x: number;
	    y: number;
	
	    static createFrom(source: any = {}) {
	        return new Placement(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.x = source["x"];
	        this.y = source["y"];
	    }
	}
	export class Board {
	    id: string;
	    name: string;
	    // Go type: time
	    created: any;
	    cards: Placement[];
	
	    static createFrom(source: any = {}) {
	        return new Board(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.created = this.convertValues(source["created"], null);
	        this.cards = this.convertValues(source["cards"], Placement);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Box {
	    id: string;
	    name: string;
	    prefix: string;
	
	    static createFrom(source: any = {}) {
	        return new Box(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.prefix = source["prefix"];
	    }
	}
	export class Drawer {
	    index: number;
	    box: string;
	    number: number;
	    label: string;
	    title: string;
	    root: string;
	    roots: number;
	    part: number;
	    parts: number;
	    first: string;
	    last: string;
	    ids: string[];
	    unsorted: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Drawer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.box = source["box"];
	        this.number = source["number"];
	        this.label = source["label"];
	        this.title = source["title"];
	        this.root = source["root"];
	        this.roots = source["roots"];
	        this.part = source["part"];
	        this.parts = source["parts"];
	        this.first = source["first"];
	        this.last = source["last"];
	        this.ids = source["ids"];
	        this.unsorted = source["unsorted"];
	    }
	}
	export class Link {
	    raw: string;
	    target: string;
	    alias: string;
	    resolved: string;
	
	    static createFrom(source: any = {}) {
	        return new Link(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.raw = source["raw"];
	        this.target = source["target"];
	        this.alias = source["alias"];
	        this.resolved = source["resolved"];
	    }
	}
	
	export class RegisterEntry {
	    term: string;
	    targets: string[];
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new RegisterEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.term = source["term"];
	        this.targets = source["targets"];
	        this.note = source["note"];
	    }
	}
	export class Settings {
	    drawerRule: string;
	    drawerSize: number;
	    deskCapacity: number;
	    boxes: Box[];
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.drawerRule = source["drawerRule"];
	        this.drawerSize = source["drawerSize"];
	        this.deskCapacity = source["deskCapacity"];
	        this.boxes = this.convertValues(source["boxes"], Box);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Warning {
	    kind: string;
	    id: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new Warning(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.id = source["id"];
	        this.message = source["message"];
	    }
	}

}

