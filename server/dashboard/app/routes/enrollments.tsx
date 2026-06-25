import { useState, useEffect } from "react";
import type { IEnrollment } from "../interfaces/IApiService";
import {
  getAllEnrollments,
  approveEnrollment,
  rejectEnrollment,
} from "../services/apiService";

const STATUS_LABELS = ["Pending", "Approved", "Rejected"];
const STATUS_COLORS = [
  "text-yellow-500",
  "text-green-500",
  "text-red-500",
];

export default function Enrollments() {
  const [enrollments, setEnrollments] = useState<IEnrollment[]>([]);
  const [loading, setLoading] = useState(true);
  const [fetchError, setFetchError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const refresh = async () => {
    setLoading(true);
    setFetchError(null);
    try {
      const data = await getAllEnrollments();
      setEnrollments(data);
    } catch (e) {
      setFetchError(String(e));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    refresh();
  }, []);

  const handleApprove = async (mac: string) => {
    setActionError(null);
    try {
      await approveEnrollment(mac);
      await refresh();
    } catch (e) {
      setActionError(String(e));
    }
  };

  const handleReject = async (mac: string) => {
    setActionError(null);
    try {
      await rejectEnrollment(mac);
      await refresh();
    } catch (e) {
      setActionError(String(e));
    }
  };

  return (
    <div className="p-4">
      <h1 className="text-2xl font-bold mb-4">Node Enrollments</h1>

      {actionError && (
        <div className="mb-4 p-3 bg-red-100 border border-red-400 text-red-700 rounded">
          <strong>Action failed:</strong> {actionError}
        </div>
      )}

      {loading ? (
        <p>Loading enrollments...</p>
      ) : fetchError ? (
        <p className="text-red-500">Error: {fetchError}</p>
      ) : enrollments.length === 0 ? (
        <p className="text-gray-500">
          No enrollment requests yet. New nodes will appear here when they first
          connect.
        </p>
      ) : (
        <table className="w-full border-collapse">
          <thead>
            <tr className="text-left border-b">
              <th className="p-2">MAC</th>
              <th className="p-2">Public Key</th>
              <th className="p-2">Status</th>
              <th className="p-2">Received</th>
              <th className="p-2">Actions</th>
            </tr>
          </thead>
          <tbody>
            {enrollments.map((e) => (
              <tr key={e.mac} className="border-b hover:bg-gray-50">
                <td className="p-2 font-mono">{e.mac}</td>
                <td className="p-2 font-mono text-xs">
                  {e.publicKey.substring(0, 16)}...
                </td>
                <td className={`p-2 font-semibold ${STATUS_COLORS[e.status]}`}>
                  {STATUS_LABELS[e.status]}
                </td>
                <td className="p-2 text-sm">
                  {new Date(e.receivedAt * 1000).toLocaleString()}
                </td>
                <td className="p-2 space-x-2">
                  {e.status === 0 && (
                    <>
                      <button
                        onClick={() => handleApprove(e.mac)}
                        className="px-3 py-1 bg-green-500 text-white rounded hover:bg-green-600"
                      >
                        Approve
                      </button>
                      <button
                        onClick={() => handleReject(e.mac)}
                        className="px-3 py-1 bg-red-500 text-white rounded hover:bg-red-600"
                      >
                        Reject
                      </button>
                    </>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      <button
        onClick={refresh}
        className="mt-4 px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600"
      >
        Refresh
      </button>
    </div>
  );
}
