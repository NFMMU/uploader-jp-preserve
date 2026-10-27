import type React from "react";
import { Children } from "react";
export function Table({
  headers,
  children,
  empty = "データがありません",
}: {
  headers: string[];
  children: React.ReactNode;
  empty?: string;
}) {
  return (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            {headers.map((h) => (
              <th key={h}>{h}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {Children.count(children) > 0 ? children : (
            <tr>
              <td colSpan={headers.length} className="empty">
                {empty}
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
