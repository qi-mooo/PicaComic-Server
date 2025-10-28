/// PicaComic 服务器客户端集成示例
/// 
/// 这个文件展示了如何在 Flutter 应用中集成服务器 API

import 'dart:convert';
import 'package:dio/dio.dart';

/// 服务器客户端
class PicaServerClient {
  final String serverUrl;
  final Dio _dio;

  PicaServerClient(this.serverUrl) : _dio = Dio(BaseOptions(
    baseUrl: serverUrl,
    connectTimeout: const Duration(seconds: 30),
    receiveTimeout: const Duration(seconds: 30),
  ));

  /// 获取所有已下载的漫画
  Future<List<ServerComic>> getComics() async {
    try {
      final response = await _dio.get('/api/comics');
      final data = response.data as Map<String, dynamic>;
      final comics = (data['comics'] as List)
          .map((e) => ServerComic.fromJson(e))
          .toList();
      return comics;
    } catch (e) {
      throw Exception('获取漫画列表失败: $e');
    }
  }

  /// 获取漫画详情
  Future<ServerComic> getComicDetail(String id) async {
    try {
      final response = await _dio.get('/api/comics/$id');
      return ServerComic.fromJson(response.data);
    } catch (e) {
      throw Exception('获取漫画详情失败: $e');
    }
  }

  /// 获取漫画封面 URL
  String getComicCoverUrl(String id) {
    return '$serverUrl/api/comics/$id/cover';
  }

  /// 获取漫画页面图片 URL
  String getComicPageUrl(String id, int ep, int page) {
    return '$serverUrl/api/comics/$id/$ep/$page';
  }

  /// 添加下载任务
  Future<DownloadTask> addDownloadTask({
    required String type,
    required String comicId,
    List<int>? eps,
  }) async {
    try {
      final response = await _dio.post('/api/download', data: {
        'type': type,
        'comic_id': comicId,
        if (eps != null) 'eps': eps,
      });
      return DownloadTask.fromJson(response.data['task']);
    } catch (e) {
      throw Exception('添加下载任务失败: $e');
    }
  }

  /// 获取下载队列
  Future<List<DownloadTask>> getDownloadQueue() async {
    try {
      final response = await _dio.get('/api/download/queue');
      final data = response.data as Map<String, dynamic>;
      final queue = (data['queue'] as List)
          .map((e) => DownloadTask.fromJson(e))
          .toList();
      return queue;
    } catch (e) {
      throw Exception('获取下载队列失败: $e');
    }
  }

  /// 开始下载
  Future<void> startDownload() async {
    await _dio.post('/api/download/start');
  }

  /// 暂停下载
  Future<void> pauseDownload() async {
    await _dio.post('/api/download/pause');
  }

  /// 取消下载任务
  Future<void> cancelDownload(String taskId) async {
    await _dio.delete('/api/download/$taskId');
  }

  /// 删除漫画
  Future<void> deleteComic(String id) async {
    await _dio.delete('/api/comics/$id');
  }

  /// PicaComic 登录
  Future<String> picacgLogin(String email, String password) async {
    try {
      final response = await _dio.post('/api/picacg/login', data: {
        'email': email,
        'password': password,
      });
      return response.data['token'];
    } catch (e) {
      throw Exception('登录失败: $e');
    }
  }

  /// PicaComic 搜索
  Future<PicacgSearchResult> picacgSearch({
    required String keyword,
    String sort = 'dd',
    int page = 1,
  }) async {
    try {
      final response = await _dio.get('/api/picacg/search', queryParameters: {
        'keyword': keyword,
        'sort': sort,
        'page': page,
      });
      return PicacgSearchResult.fromJson(response.data);
    } catch (e) {
      throw Exception('搜索失败: $e');
    }
  }

  /// PicaComic 获取漫画信息
  Future<PicacgComic> picacgGetComic(String id) async {
    try {
      final response = await _dio.get('/api/picacg/comic/$id');
      return PicacgComic.fromJson(response.data);
    } catch (e) {
      throw Exception('获取漫画信息失败: $e');
    }
  }

  /// PicaComic 获取章节
  Future<List<String>> picacgGetEps(String id) async {
    try {
      final response = await _dio.get('/api/picacg/comic/$id/eps');
      return List<String>.from(response.data['eps']);
    } catch (e) {
      throw Exception('获取章节失败: $e');
    }
  }
}

/// 服务器漫画模型
class ServerComic {
  final String id;
  final String title;
  final String author;
  final String description;
  final String cover;
  final List<String> tags;
  final List<String> categories;
  final int epsCount;
  final int pagesCount;
  final String type;
  final DateTime time;
  final int size;
  final List<String>? eps;
  final List<int>? downloadedEps;
  final String? directory;

  ServerComic({
    required this.id,
    required this.title,
    required this.author,
    required this.description,
    required this.cover,
    required this.tags,
    required this.categories,
    required this.epsCount,
    required this.pagesCount,
    required this.type,
    required this.time,
    required this.size,
    this.eps,
    this.downloadedEps,
    this.directory,
  });

  factory ServerComic.fromJson(Map<String, dynamic> json) {
    return ServerComic(
      id: json['id'],
      title: json['title'],
      author: json['author'] ?? '',
      description: json['description'] ?? '',
      cover: json['cover'] ?? '',
      tags: List<String>.from(json['tags'] ?? []),
      categories: List<String>.from(json['categories'] ?? []),
      epsCount: json['eps_count'] ?? 0,
      pagesCount: json['pages_count'] ?? 0,
      type: json['type'] ?? '',
      time: DateTime.fromMillisecondsSinceEpoch(
        (json['time'] ?? 0) * 1000,
      ),
      size: json['size'] ?? 0,
      eps: json['eps'] != null ? List<String>.from(json['eps']) : null,
      downloadedEps: json['downloaded_eps'] != null 
          ? List<int>.from(json['downloaded_eps']) 
          : null,
      directory: json['directory'],
    );
  }
}

/// 下载任务模型
class DownloadTask {
  final String id;
  final String comicId;
  final String title;
  final String type;
  final String cover;
  final int totalPages;
  final int downloadedPages;
  final int currentEp;
  final String status;
  final String? error;
  final DateTime createdAt;
  final DateTime updatedAt;

  DownloadTask({
    required this.id,
    required this.comicId,
    required this.title,
    required this.type,
    required this.cover,
    required this.totalPages,
    required this.downloadedPages,
    required this.currentEp,
    required this.status,
    this.error,
    required this.createdAt,
    required this.updatedAt,
  });

  factory DownloadTask.fromJson(Map<String, dynamic> json) {
    return DownloadTask(
      id: json['id'],
      comicId: json['comic_id'],
      title: json['title'],
      type: json['type'],
      cover: json['cover'] ?? '',
      totalPages: json['total_pages'] ?? 0,
      downloadedPages: json['downloaded_pages'] ?? 0,
      currentEp: json['current_ep'] ?? 0,
      status: json['status'],
      error: json['error'],
      createdAt: DateTime.parse(json['created_at']),
      updatedAt: DateTime.parse(json['updated_at']),
    );
  }

  double get progress {
    if (totalPages == 0) return 0;
    return downloadedPages / totalPages;
  }
}

/// PicaComic 搜索结果
class PicacgSearchResult {
  final List<PicacgComicBrief> comics;
  final int pages;

  PicacgSearchResult({
    required this.comics,
    required this.pages,
  });

  factory PicacgSearchResult.fromJson(Map<String, dynamic> json) {
    return PicacgSearchResult(
      comics: [], // 需要解析 JSON
      pages: 0,
    );
  }
}

/// PicaComic 漫画简要信息
class PicacgComicBrief {
  final String id;
  final String title;
  final String author;
  final String cover;

  PicacgComicBrief({
    required this.id,
    required this.title,
    required this.author,
    required this.cover,
  });
}

/// PicaComic 漫画详情
class PicacgComic {
  final String id;
  final String title;
  final String author;
  final String description;
  final List<String> tags;
  final List<String> categories;

  PicacgComic({
    required this.id,
    required this.title,
    required this.author,
    required this.description,
    required this.tags,
    required this.categories,
  });

  factory PicacgComic.fromJson(Map<String, dynamic> json) {
    return PicacgComic(
      id: json['_id'],
      title: json['title'],
      author: json['author'] ?? '',
      description: json['description'] ?? '',
      tags: List<String>.from(json['tags'] ?? []),
      categories: List<String>.from(json['categories'] ?? []),
    );
  }
}

/// 使用示例
void main() async {
  // 创建客户端（使用服务器地址）
  final client = PicaServerClient('http://192.168.1.100:8080');

  // 获取已下载的漫画
  final comics = await client.getComics();
  print('已下载 ${comics.length} 部漫画');

  // 登录 PicaComic
  await client.picacgLogin('email@example.com', 'password');

  // 搜索漫画
  final searchResult = await client.picacgSearch(
    keyword: '测试',
    sort: 'dd',
    page: 1,
  );

  // 添加下载任务
  final task = await client.addDownloadTask(
    type: 'picacg',
    comicId: 'some-comic-id',
    eps: [1, 2, 3],
  );

  // 开始下载
  await client.startDownload();

  // 获取下载队列
  final queue = await client.getDownloadQueue();
  for (final task in queue) {
    print('${task.title}: ${(task.progress * 100).toStringAsFixed(1)}%');
  }
}

