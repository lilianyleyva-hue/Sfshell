import Foundation
#if canImport(FoundationXML)
import FoundationXML
#endif

// ============================================================
// MARK: - Utilidades de datos (JSON / XML / plist)
// ============================================================

enum DataTools {

    static func jsonPretty(_ text: String) throws -> String {
        guard let d = text.data(using: .utf8) else { throw ShErr("json: texto inválido") }
        let obj = try JSONSerialization.jsonObject(with: d, options: [.fragmentsAllowed])
        let out = try JSONSerialization.data(withJSONObject: obj, options: [.prettyPrinted, .sortedKeys, .fragmentsAllowed])
        return String(data: out, encoding: .utf8) ?? ""
    }

    static func jsonMin(_ text: String) throws -> String {
        guard let d = text.data(using: .utf8) else { throw ShErr("json: texto inválido") }
        let obj = try JSONSerialization.jsonObject(with: d, options: [.fragmentsAllowed])
        let out = try JSONSerialization.data(withJSONObject: obj, options: [.fragmentsAllowed])
        return String(data: out, encoding: .utf8) ?? ""
    }

    static func jsonValue(_ text: String, path: String) throws -> Any {
        guard let d = text.data(using: .utf8) else { throw ShErr("json: texto inválido") }
        var node = try JSONSerialization.jsonObject(with: d, options: [.fragmentsAllowed])
        var token = ""
        var tokens: [String] = []
        for ch in path {
            if ch == "." { if !token.isEmpty { tokens.append(token); token = "" } }
            else if ch == "[" { if !token.isEmpty { tokens.append(token); token = "" } }
            else if ch == "]" { tokens.append("#" + token); token = "" }
            else { token.append(ch) }
        }
        if !token.isEmpty { tokens.append(token) }

        for t in tokens where !t.isEmpty {
            if t.hasPrefix("#") {
                guard let arr = node as? [Any], let idx = Int(t.dropFirst()), idx >= 0, idx < arr.count else {
                    throw ShErr("json: índice fuera de rango en '\(t.dropFirst())'")
                }
                node = arr[idx]
            } else {
                guard let dict = node as? [String: Any], let v = dict[t] else {
                    throw ShErr("json: no existe la clave '\(t)'")
                }
                node = v
            }
        }
        return node
    }

    static func describe(_ v: Any) -> String {
        if let s = v as? String { return s }
        if JSONSerialization.isValidJSONObject(v),
           let d = try? JSONSerialization.data(withJSONObject: v, options: [.prettyPrinted, .sortedKeys]),
           let s = String(data: d, encoding: .utf8) { return s }
        return String(describing: v)
    }

    static func plistShow(_ data: Data) throws -> String {
        let obj = try PropertyListSerialization.propertyList(from: data, options: [], format: nil)
        return describe(obj)
    }

    static func plistToJSON(_ data: Data) throws -> String {
        let obj = try PropertyListSerialization.propertyList(from: data, options: [], format: nil)
        guard JSONSerialization.isValidJSONObject(obj) else { throw ShErr("plist: contiene datos que JSON no admite (Data o Date)") }
        let out = try JSONSerialization.data(withJSONObject: obj, options: [.prettyPrinted, .sortedKeys])
        return String(data: out, encoding: .utf8) ?? ""
    }

    static func jsonToPlist(_ text: String) throws -> String {
        guard let d = text.data(using: .utf8) else { throw ShErr("texto inválido") }
        let obj = try JSONSerialization.jsonObject(with: d, options: [])
        let out = try PropertyListSerialization.data(fromPropertyList: obj, format: .xml, options: 0)
        return String(data: out, encoding: .utf8) ?? ""
    }
}

// Árbol XML mínimo
final class XMLNode {
    var name: String
    var attrs: [String: String]
    var text: String = ""
    var children: [XMLNode] = []
    weak var parent: XMLNode?
    init(name: String, attrs: [String: String] = [:]) { self.name = name; self.attrs = attrs }

    func pretty(_ level: Int = 0) -> String {
        let pad = String(repeating: "  ", count: level)
        let a = attrs.sorted { $0.key < $1.key }.map { " \($0.key)=\"\($0.value)\"" }.joined()
        let body = text.trimmingCharacters(in: .whitespacesAndNewlines)
        if children.isEmpty {
            return body.isEmpty ? "\(pad)<\(name)\(a)/>" : "\(pad)<\(name)\(a)>\(body)</\(name)>"
        }
        var s = "\(pad)<\(name)\(a)>"
        if !body.isEmpty { s += "\n\(pad)  \(body)" }
        for c in children { s += "\n" + c.pretty(level + 1) }
        s += "\n\(pad)</\(name)>"
        return s
    }

    func allText() -> String {
        var s = text.trimmingCharacters(in: .whitespacesAndNewlines)
        for c in children {
            let t = c.allText()
            if !t.isEmpty { s += (s.isEmpty ? "" : "\n") + t }
        }
        return s
    }

    func find(_ tag: String, into acc: inout [XMLNode]) {
        if name == tag { acc.append(self) }
        for c in children { c.find(tag, into: &acc) }
    }
}

final class XMLTreeBuilder: NSObject, XMLParserDelegate {
    var rootNode: XMLNode?
    private var current: XMLNode?
    var failure: String?

    func parse(_ data: Data) throws -> XMLNode {
        let p = XMLParser(data: data)
        p.delegate = self
        guard p.parse(), let r = rootNode else {
            throw ShErr("xml: \(failure ?? p.parserError?.localizedDescription ?? "no se pudo analizar")")
        }
        return r
    }

    func parser(_ parser: XMLParser, didStartElement e: String, namespaceURI: String?,
                qualifiedName qn: String?, attributes a: [String: String] = [:]) {
        let n = XMLNode(name: e, attrs: a)
        n.parent = current
        if let c = current { c.children.append(n) } else { rootNode = n }
        current = n
    }

    func parser(_ parser: XMLParser, foundCharacters s: String) { current?.text += s }

    func parser(_ parser: XMLParser, didEndElement e: String, namespaceURI: String?, qualifiedName qn: String?) {
        current = current?.parent
    }

    func parser(_ parser: XMLParser, parseErrorOccurred e: Error) { failure = e.localizedDescription }
}
